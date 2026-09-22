package harness

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// DevOptions 是 Dev 编排器的参数。
type DevOptions struct {
	Pkg      string        // 业务包路径（go build 的参数形态，如 ./cmd/demo）
	WorkDir  string        // 构建与业务进程的工作目录
	StateDir string        // 状态目录（socket、构建产物）；默认 <WorkDir>/.nib
	Interval time.Duration // 文件轮询间隔；默认 250ms
	NoWatch  bool          // 只构建运行一次，不监听改动
}

// Dev 运行开发常驻外壳：构建业务二进制，拉起 shell（常驻窗口）与业务进程，
// 监听 Go 源改动并热替换业务进程（窗口不退出、不闪烁，前端状态保留）。
// 阻塞至收到 SIGINT/SIGTERM 或窗口关闭。
func Dev(opts DevOptions) error {
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

	// 1. 构建业务二进制。
	if err := buildApp(opts.WorkDir, opts.Pkg, binPath); err != nil {
		return fmt.Errorf("首次构建失败：%w", err)
	}

	// 2. 拉起常驻 shell（窗口永不退出的那一层）。
	shell := exec.Command(self, "shell", "--sock", sockPath)
	shell.Dir = opts.WorkDir
	shell.Stdout = os.Stdout
	shell.Stderr = os.Stderr
	shell.Env = os.Environ()
	if err := shell.Start(); err != nil {
		return fmt.Errorf("启动 shell: %w", err)
	}

	// 3. 拉起业务进程（可替换层）。
	app, err := startApp(binPath, sockPath, opts.WorkDir)
	if err != nil {
		shell.Process.Kill()
		return err
	}

	log.Printf("nib dev: 已启动（pkg=%s）。改 Go 源码即热替换业务进程。", opts.Pkg)

	// 4. 监听改动 → 重建 → 重启业务进程。
	var changes <-chan struct{}
	var stopWatch func()
	if !opts.NoWatch {
		changes, stopWatch = WatchChanges(opts.WorkDir, opts.Interval)
		defer stopWatch()
	}
	restarting := false

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-changes:
			if restarting {
				continue
			}
			restarting = true
			go func() {
				defer func() { restarting = false }()
				if err := buildApp(opts.WorkDir, opts.Pkg, binPath); err != nil {
					log.Printf("nib dev: 构建失败，保留旧进程：%v", err)
					return
				}
				stopApp(app)
				next, err := startApp(binPath, sockPath, opts.WorkDir)
				if err != nil {
					log.Printf("nib dev: 重启业务进程失败: %v", err)
					return
				}
				app = next
				log.Printf("nib dev: 业务进程已热替换")
			}()
		case s := <-sig:
			log.Printf("nib dev: 收到 %v，退出", s)
			stopApp(app)
			shell.Process.Signal(syscall.SIGTERM)
			shell.Wait()
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
	cmd := exec.Command(binPath)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

// RunShell 是 `nib shell --sock <path>` 的实现：创建常驻窗口并服务业务进程。
// 由 Dev 经子进程拉起，业务用户不需要直接调用。
func RunShell(sockPath string) error {
	os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", sockPath, err)
	}
	defer os.Remove(sockPath)
	return serveShell(ln)
}
