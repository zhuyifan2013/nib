package harness

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"nib.dev/nib/cli"
	"nib.dev/nib/driver"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// DevOptions 是 Dev 编排器的参数。
type DevOptions struct {
	ChildOutput io.Writer
	ChildError  io.Writer
	Notify      func(string, any)
	Headless    bool
	Driver      driver.Options
	Pkg         string        // 业务包路径（go build 的参数形态，如 ./cmd/demo）
	WorkDir     string        // 构建与业务进程的工作目录
	StateDir    string        // 状态目录（socket、构建产物）；默认 <WorkDir>/.nib
	Interval    time.Duration // 文件轮询间隔；默认 250ms
	NoWatch     bool          // 只构建运行一次，不监听改动
	Events      string        // 事件流 NDJSON 导出路径（"-" 为 stdout；空 = 不导出）
}

// Dev 运行开发常驻外壳：构建业务二进制，拉起 shell（常驻窗口）与业务进程，
// 监听 Go 源改动并热替换业务进程（窗口不退出、不闪烁，前端状态保留）。
// 阻塞至收到 SIGINT/SIGTERM 或窗口关闭。
func Dev(opts DevOptions) error {
	if err := driver.ValidateTimeout(opts.Driver.Timeout); err != nil {
		return err
	}
	if opts.Pkg == "" {
		return fmt.Errorf("缺少业务包路径（如 ./cmd/demo）")
	}
	if opts.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		opts.WorkDir = wd
	}
	if opts.StateDir == "" {
		opts.StateDir = filepath.Join(opts.WorkDir, ".nib")
	}
	if opts.Interval == 0 {
		opts.Interval = 250 * time.Millisecond
	}
	if err := os.MkdirAll(opts.StateDir, 0o755); err != nil {
		return err
	}
	binPath := filepath.Join(opts.StateDir, "dev-app")
	sockPath := filepath.Join(opts.StateDir, "shell.sock")
	os.Remove(sockPath)

	self, err := os.Executable()
	if err != nil {
		return err
	}

	if opts.ChildOutput == nil {
		opts.ChildOutput = os.Stdout
	}
	if opts.ChildError == nil {
		opts.ChildError = os.Stderr
	}
	notify := func(status string, data any) {
		if opts.Notify != nil {
			opts.Notify(status, data)
		}
	}
	notify("building", nil)
	// 1. 构建业务二进制。
	if err := buildApp(opts.WorkDir, opts.Pkg, binPath); err != nil {
		return cli.Fail("dev/build", "Fix compiler errors in the package; inspect stderr or the error message.", err)
	}

	// 2. 拉起常驻 shell（窗口永不退出的那一层）。
	shellArgs, err := devShellArgs(sockPath, opts)
	if err != nil {
		return err
	}
	shell := exec.Command(self, shellArgs...)
	shell.Dir = opts.WorkDir
	shell.Stdout = opts.ChildOutput
	shell.Stderr = opts.ChildError
	shell.Env = os.Environ()
	if opts.Driver.Token != "" {
		shell.Env = append(shell.Env, "NIB_DRIVER_TOKEN="+opts.Driver.Token)
	}
	if err := shell.Start(); err != nil {
		return fmt.Errorf("启动 shell: %w", err)
	}

	shellDone := make(chan error, 1)
	go func() { shellDone <- shell.Wait() }()

	// Wait for the resident listener before the app attempts its first DialRemote.
	if err := waitShellSocket(sockPath, shellDone, 5*time.Second); err != nil {
		shell.Process.Kill()
		return err
	}

	// 3. 拉起业务进程（可替换层）。
	app, err := startAppWithOutput(binPath, sockPath, opts.WorkDir, opts.ChildOutput, opts.ChildError)
	if err != nil {
		shell.Process.Kill()
		<-shellDone
		return err
	}

	notify("started", map[string]any{"driver_port": opts.Driver.Port})
	log.Printf("nib dev: 已启动（pkg=%s）。改 Go 源码即热替换业务进程。", opts.Pkg)

	// 4. 监听改动 → 重建 → 重启业务进程。
	var changes <-chan struct{}
	var stopWatch func()
	if !opts.NoWatch {
		changes, stopWatch = WatchChanges(opts.WorkDir, opts.Interval)
		defer stopWatch()
	}
	restarting := false
	built := make(chan error, 1)

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	for {
		select {
		case <-changes:
			if restarting {
				continue
			}
			restarting = true
			notify("rebuilding", nil)
			go func() { built <- buildApp(opts.WorkDir, opts.Pkg, binPath) }()
		case err := <-built:
			restarting = false
			if err != nil {
				notify("rebuild_failed", map[string]any{"code": "dev/build"})
				log.Printf("nib dev: 构建失败，保留旧进程：%v", err)
				continue
			}
			stopApp(app)
			next, err := startAppWithOutput(binPath, sockPath, opts.WorkDir, opts.ChildOutput, opts.ChildError)
			if err != nil {
				log.Printf("nib dev: 重启业务进程失败: %v", err)
				continue
			}
			app = next
			notify("restarted", nil)
			log.Printf("nib dev: 业务进程已热替换")
		case err := <-shellDone:
			stopApp(app)
			os.Remove(sockPath)
			return err
		case s := <-sig:
			log.Printf("nib dev: 收到 %v，退出", s)
			stopApp(app)
			shell.Process.Signal(syscall.SIGTERM)
			<-shellDone
			os.Remove(sockPath)
			return nil
		}
	}
}

func buildApp(workDir, pkg, binPath string) error {
	cmd := exec.Command("go", "build", "-o", binPath, pkg)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, out)
	}
	return nil
}

func startApp(binPath, sockPath, workDir string) (*exec.Cmd, error) {
	return startAppWithOutput(binPath, sockPath, workDir, os.Stdout, os.Stderr)
}
func startAppWithOutput(binPath, sockPath, workDir string, out, errOut io.Writer) (*exec.Cmd, error) {
	cmd := exec.Command(binPath)
	cmd.Dir = workDir
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Env = append(os.Environ(), "NIB_SOCK="+sockPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动业务进程: %w", err)
	}
	return cmd, nil
}

func stopApp(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		<-done
	}
}

// RunShell 是 `nib shell --sock <path> [--events <path|->]` 的实现：创建常驻窗口并服务业务进程。
// 由 Dev 经子进程拉起，业务用户不需要直接调用。
func RunShell(sockPath, eventsPath string) error {
	os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", sockPath, err)
	}
	defer os.Remove(sockPath)
	defer ln.Close()
	return serveShell(ln, eventsPath, ShellOptions{})
}

// ShellOptions configures explicitly enabled local automation on the resident shell.
type ShellOptions struct {
	Context  context.Context
	Notify   func(string, any)
	Headless bool
	Driver   driver.Options
}

func RunShellWithOptions(sockPath, eventsPath string, opts ShellOptions) error {
	if err := driver.ValidateTimeout(opts.Driver.Timeout); err != nil {
		return err
	}
	os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(sockPath)
	return serveShell(ln, eventsPath, opts)
}

func waitShellSocket(path string, exited <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		select {
		case err := <-exited:
			if err == nil {
				return fmt.Errorf("shell exited before its socket became ready")
			}
			return fmt.Errorf("shell startup failed: %w", err)
		case <-timer.C:
			return fmt.Errorf("shell socket readiness timed out")
		case <-ticker.C:
		}
	}
}

// devShellArgs preserves automation options across the resident-shell process boundary.
// The token is intentionally carried only by the environment.
func devShellArgs(sockPath string, opts DevOptions) ([]string, error) {
	if err := driver.ValidateTimeout(opts.Driver.Timeout); err != nil {
		return nil, err
	}
	args := []string{"shell", "--sock", sockPath}
	if opts.Events != "" {
		args = append(args, "--events", opts.Events)
	}
	if opts.Headless {
		args = append(args, "--headless")
	}
	if opts.Driver.Port != 0 {
		args = append(args, "--driver-port", fmt.Sprint(opts.Driver.Port), "--driver-timeout", opts.Driver.Timeout.String())
	}
	return args, nil
}
