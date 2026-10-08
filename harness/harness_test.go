package harness

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"nib.dev/nib/driver"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nib.dev/nib/events"
	"nib.dev/nib/ipc"
)

// shortSock 返回短路径 unix socket（t.TempDir 全路径会超出 macOS sun_path 104 字节限制）。
func shortSock(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.TempDir(), fmt.Sprintf("nib-test-%d.sock", time.Now().UnixNano()))
}

// fakeWindow 记录 shell 对窗口的全部操作。
type fakeWindow struct {
	mu        sync.Mutex
	forward   func(*ipc.Message)
	navigated []string
	titles    []string
	resized   [][2]int
	evals     []string
	delivered []*ipc.Message
	closed    bool
	runDone   chan struct{}
	bus       *events.Bus
	closeOnce sync.Once
}

func newFakeWindow() *fakeWindow {
	return &fakeWindow{runDone: make(chan struct{}), bus: events.NewBus(16)}
}

func (f *fakeWindow) SetForward(fn func(*ipc.Message)) { f.forward = fn }
func (f *fakeWindow) Events() *events.Bus              { return f.bus }
func (f *fakeWindow) DeliverResponse(m *ipc.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delivered = append(f.delivered, m)
}
func (f *fakeWindow) Navigate(t string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.navigated = append(f.navigated, t)
	return nil
}
func (f *fakeWindow) Eval(js string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.evals = append(f.evals, js)
	return nil
}
func (f *fakeWindow) SetTitle(t string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.titles = append(f.titles, t)
}
func (f *fakeWindow) Resize(w, h int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resized = append(f.resized, [2]int{w, h})
}
func (f *fakeWindow) Run() error { <-f.runDone; return nil }
func (f *fakeWindow) Close() error {
	f.closeOnce.Do(func() { f.mu.Lock(); f.closed = true; f.mu.Unlock(); close(f.runDone) })
	return nil
}

func (f *fakeWindow) snapshot() fakeWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeWindow{navigated: append([]string(nil), f.navigated...), titles: append([]string(nil), f.titles...), evals: append([]string(nil), f.evals...), resized: append([][2]int(nil), f.resized...), delivered: append([]*ipc.Message(nil), f.delivered...), closed: f.closed}
}

// startTestShell 在临时目录起 unix socket + Shell，返回 fake 窗口与清理函数。
func startTestShell(t *testing.T) (*fakeWindow, *Shell, func()) {
	t.Helper()
	sock := shortSock(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	win := newFakeWindow()
	sh := NewShell(win, ln)
	go sh.Run()
	return win, sh, func() { sh.Close() }
}

// dialApp 模拟业务进程：连上 shell 并完成 init 握手。
func dialApp(t *testing.T, sock, navigate string, bindings ...string) net.Conn {
	t.Helper()
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	init := initPayload{Navigate: navigate, Bindings: bindings}
	if err := Send(conn, &ipc.Message{ID: "init", Dir: ipc.DirRequest, Call: CtrlInit, Args: marshalArgs(init)}); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestShellForwardsCallsToApp(t *testing.T) {
	sock := shortSock(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	win := newFakeWindow()
	sh := NewShell(win, ln)
	go sh.Run()
	defer sh.Close()

	app := dialApp(t, sock, "<html>demo</html>", "Svc.M")
	defer app.Close()
	waitFor(t, func() bool { return len(win.snapshot().navigated) == 1 })

	// 前端调用 → shell 转发给业务进程。
	// （shell 会穿插推送事件流 CtrlEvents，recvRequest 跳过它们。）
	win.forward(&ipc.Message{ID: "m1", Dir: ipc.DirRequest, Call: "Svc.M", Args: marshalArgs(map[string]any{"x": 1})})
	req := recvRequest(t, app)
	if req.Call != "Svc.M" || req.ID != "m1" {
		t.Fatalf("转发消息不符: %+v", req)
	}

	// 业务进程响应 → shell 送回前端。
	if err := Send(app, &ipc.Message{ID: "m1", Dir: ipc.DirResponse, Result: marshalArgs("hi")}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(win.snapshot().delivered) == 1 })
	if got := string(win.snapshot().delivered[0].Result); got != `"hi"` {
		t.Fatalf("响应结果 = %s", got)
	}
}

func TestShellAppOfflineGivesRetryableError(t *testing.T) {
	win, _, cleanup := startTestShell(t)
	defer cleanup()

	win.forward(&ipc.Message{ID: "m1", Dir: ipc.DirRequest, Call: "Svc.M"})
	waitFor(t, func() bool { return len(win.snapshot().delivered) == 1 })
	e := win.snapshot().delivered[0].Error
	if e == nil || e.Code != errAppDown || !e.Retryable {
		t.Fatalf("应返回可重试的 %s，实际 %+v", errAppDown, e)
	}
}

func TestShellCtrlMessages(t *testing.T) {
	win, _, cleanup := startTestShell(t)
	defer cleanup()

	win.forward(&ipc.Message{ID: "c1", Dir: ipc.DirRequest, Call: CtrlSetTitle, Args: marshalArgs("你好")})
	win.forward(&ipc.Message{ID: "c2", Dir: ipc.DirRequest, Call: CtrlResize, Args: marshalArgs(struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}{800, 600})})
	win.forward(&ipc.Message{ID: "c3", Dir: ipc.DirRequest, Call: CtrlEval, Args: marshalArgs("1+1")})
	waitFor(t, func() bool {
		return len(win.snapshot().titles) == 1 && len(win.snapshot().resized) == 1 && len(win.snapshot().evals) == 1
	})
	if win.snapshot().titles[0] != "你好" || win.snapshot().resized[0] != [2]int{800, 600} || win.snapshot().evals[0] != "1+1" {
		t.Fatalf("控制消息未生效: %+v %+v %+v", win.snapshot().titles, win.snapshot().resized, win.snapshot().evals)
	}
}

func TestRemoteWindowEndToEnd(t *testing.T) {
	sock := shortSock(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	win := newFakeWindow()
	sh := NewShell(win, ln)
	go sh.Run()
	defer sh.Close()

	// 业务侧：remote 窗口注册绑定 + 起始页面，然后 Run 服务调用。
	appBus := events.NewBus(64)
	remote, err := DialRemote(sock, appBus)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Bind("Svc.Greet", func(args json.RawMessage) (any, *ipc.Error) {
		return "hello", nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := remote.Navigate("<html>page</html>"); err != nil {
		t.Fatal(err)
	}
	runErr := make(chan error, 1)
	go func() { runErr <- remote.Run() }()

	// shell 应完成 init：导航到起始页面。
	waitFor(t, func() bool {
		return len(win.snapshot().navigated) == 1 && win.snapshot().navigated[0] == "<html>page</html>"
	})

	// 前端调用 → remote handler → 响应回前端。
	win.forward(&ipc.Message{ID: "m9", Dir: ipc.DirRequest, Call: "Svc.Greet"})
	waitFor(t, func() bool { return len(win.snapshot().delivered) == 1 })
	if got := string(win.snapshot().delivered[0].Result); got != `"hello"` {
		t.Fatalf("结果 = %s", got)
	}

	// 关闭：remote 发 CtrlClose，shell 关窗，Run 返回。
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return win.snapshot().closed })
	select {
	case <-runErr:
	case <-time.After(2 * time.Second):
		t.Fatal("remote.Run 未返回")
	}
}

// recvRequest 从业务进程连接读消息，跳过事件流推送（CtrlEvents），直到一条请求。
// 用 Session 读：每条消息新建 Decoder 会丢弃其缓冲中的后续消息（M2 修过的坑）。
func recvRequest(t *testing.T, c net.Conn) *ipc.Message {
	t.Helper()
	sess := NewSession(c)
	for {
		m, err := sess.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if m.Call != CtrlEvents {
			return m
		}
	}
}

// TestShellStreamsAndReplaysEvents 验证事件流跨进程语义：
// 上线前的事件重放、在线实时转发（保留原 Seq）、离线期间缓冲、重连补齐。
func TestShellStreamsAndReplaysEvents(t *testing.T) {
	sock := shortSock(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	win := newFakeWindow()
	sh := NewShell(win, ln)
	go sh.Run()
	defer sh.Close()

	// 业务进程上线前的窗口事件（模拟 webview 创建）。
	win.bus.Emit(events.WindowCreated, map[string]any{"title": "T"})

	// 第一个业务进程上线：应收到重放 + app_connected。
	bus1 := events.NewBus(64)
	r1, err := DialRemote(sock, bus1)
	if err != nil {
		t.Fatal(err)
	}
	go r1.Run()
	waitFor(t, func() bool { return hasKind(bus1.Recent(), events.AppConnected) })
	if !hasKind(bus1.Recent(), events.WindowCreated) {
		t.Fatal("上线前事件未被重放")
	}

	// 在线实时转发，且保留 shell 侧原始 Seq。
	wantSeq := win.bus.Emit(events.WindowNavigated, map[string]any{"target": "one"}).Seq
	waitFor(t, func() bool {
		for _, ev := range bus1.Recent() {
			if ev.Kind == events.WindowNavigated && ev.Data["target"] == "one" {
				return true
			}
		}
		return false
	})
	for _, ev := range bus1.Recent() {
		if ev.Kind == events.WindowNavigated && ev.Data["target"] == "one" && ev.Seq != wantSeq {
			t.Fatalf("转发事件 Seq = %d, want %d（应保留原始 Seq）", ev.Seq, wantSeq)
		}
	}

	// 业务进程下线；离线期间产生窗口事件。
	if err := r1.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return hasKind(win.bus.Recent(), events.AppDisconnected) })
	win.bus.Emit(events.WindowNavigated, map[string]any{"target": "offline"})

	// 第二个业务进程上线：离线期间的事件应被补齐。
	bus2 := events.NewBus(64)
	r2, err := DialRemote(sock, bus2)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	go r2.Run()
	waitFor(t, func() bool {
		for _, ev := range bus2.Recent() {
			if ev.Kind == events.WindowNavigated && ev.Data["target"] == "offline" {
				return true
			}
		}
		return false
	})
}

func hasKind(evs []events.Event, kind events.Kind) bool {
	for _, ev := range evs {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待超时")
}

func TestShellSocketReadiness(t *testing.T) {
	path := shortSock(t)
	exited := make(chan error, 1)
	exited <- fmt.Errorf("startup failed")
	if err := waitShellSocket(path, exited, time.Second); err == nil {
		t.Fatal("accepted exited shell")
	}
	if err := waitShellSocket(path, make(chan error), 5*time.Millisecond); err == nil {
		t.Fatal("did not time out")
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := waitShellSocket(path, make(chan error), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestDevDriverTimeoutPropagation(t *testing.T) {
	for _, timeout := range []time.Duration{0, 1500 * time.Millisecond, 30 * time.Second} {
		opts := DevOptions{Headless: true, Events: "events.ndjson", Driver: driver.Options{Port: 8123, Timeout: timeout, Token: "private-test-token"}}
		args, err := devShellArgs("test.sock", opts)
		if err != nil {
			t.Fatal(err)
		}
		fs := flag.NewFlagSet("shell", flag.ContinueOnError)
		port := fs.Int("driver-port", 0, "")
		gotTimeout := fs.Duration("driver-timeout", 0, "")
		headless := fs.Bool("headless", false, "")
		sock := fs.String("sock", "", "")
		events := fs.String("events", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			t.Fatal(err)
		}
		if *port != opts.Driver.Port || *gotTimeout != timeout || !*headless || *sock != "test.sock" || *events != opts.Events {
			t.Fatalf("options lost: %v", args)
		}
		if strings.Contains(strings.Join(args, " "), opts.Driver.Token) {
			t.Fatal("token leaked into argv")
		}
	}
}
func TestInvalidDriverTimeoutRejectedBeforeStartup(t *testing.T) {
	for _, timeout := range []time.Duration{-time.Nanosecond, 30*time.Second + time.Nanosecond} {
		opts := DevOptions{Pkg: "nonexistent-package", Driver: driver.Options{Port: 8123, Timeout: timeout}}
		if _, err := devShellArgs("test.sock", opts); err == nil {
			t.Fatal("invalid timeout forwarded")
		}
		if err := Dev(opts); err == nil || !strings.Contains(err.Error(), "driver timeout") {
			t.Fatalf("did not reject before build: %v", err)
		}
		if err := RunShellWithOptions("", "", ShellOptions{Driver: opts.Driver}); err == nil || !strings.Contains(err.Error(), "driver timeout") {
			t.Fatalf("did not reject before socket creation: %v", err)
		}
	}
}
