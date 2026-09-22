package harness

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nib.dev/nib/ipc"
)

// shortSock 返回短路径 unix socket（t.TempDir 全路径会超出 macOS sun_path 104 字节限制）。
func shortSock(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.TempDir(), fmt.Sprintf("nib-test-%d.sock", time.Now().UnixNano()))
}

// fakeWindow 记录 shell 对窗口的全部操作。
type fakeWindow struct {
	forward   func(*ipc.Message)
	navigated []string
	titles    []string
	resized   [][2]int
	evals     []string
	delivered []*ipc.Message
	closed    bool
	runDone   chan struct{}
}

func newFakeWindow() *fakeWindow {
	return &fakeWindow{runDone: make(chan struct{})}
}

func (f *fakeWindow) SetForward(fn func(*ipc.Message)) { f.forward = fn }
func (f *fakeWindow) DeliverResponse(m *ipc.Message)   { f.delivered = append(f.delivered, m) }
func (f *fakeWindow) Navigate(t string) error          { f.navigated = append(f.navigated, t); return nil }
func (f *fakeWindow) Eval(js string) error             { f.evals = append(f.evals, js); return nil }
func (f *fakeWindow) SetTitle(t string)                { f.titles = append(f.titles, t) }
func (f *fakeWindow) Resize(w, h int)                  { f.resized = append(f.resized, [2]int{w, h}) }
func (f *fakeWindow) Run() error                       { <-f.runDone; return nil }
func (f *fakeWindow) Close() error                     { f.closed = true; close(f.runDone); return nil }

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
	return win, sh, func() { ln.Close() }
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
	defer ln.Close()

	app := dialApp(t, sock, "<html>demo</html>", "Svc.M")
	defer app.Close()
	waitFor(t, func() bool { return len(win.navigated) == 1 })

	// 前端调用 → shell 转发给业务进程。
	win.forward(&ipc.Message{ID: "m1", Dir: ipc.DirRequest, Call: "Svc.M", Args: marshalArgs(map[string]any{"x": 1})})
	req, err := Recv(app)
	if err != nil {
		t.Fatal(err)
	}
	if req.Call != "Svc.M" || req.ID != "m1" {
		t.Fatalf("转发消息不符: %+v", req)
	}

	// 业务进程响应 → shell 送回前端。
	if err := Send(app, &ipc.Message{ID: "m1", Dir: ipc.DirResponse, Result: marshalArgs("hi")}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(win.delivered) == 1 })
	if got := string(win.delivered[0].Result); got != `"hi"` {
		t.Fatalf("响应结果 = %s", got)
	}
}

func TestShellAppOfflineGivesRetryableError(t *testing.T) {
	win, _, cleanup := startTestShell(t)
	defer cleanup()

	win.forward(&ipc.Message{ID: "m1", Dir: ipc.DirRequest, Call: "Svc.M"})
	waitFor(t, func() bool { return len(win.delivered) == 1 })
	e := win.delivered[0].Error
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
		return len(win.titles) == 1 && len(win.resized) == 1 && len(win.evals) == 1
	})
	if win.titles[0] != "你好" || win.resized[0] != [2]int{800, 600} || win.evals[0] != "1+1" {
		t.Fatalf("控制消息未生效: %+v %+v %+v", win.titles, win.resized, win.evals)
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
	defer ln.Close()

	// 业务侧：remote 窗口注册绑定 + 起始页面，然后 Run 服务调用。
	remote, err := DialRemote(sock)
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
	waitFor(t, func() bool { return len(win.navigated) == 1 && win.navigated[0] == "<html>page</html>" })

	// 前端调用 → remote handler → 响应回前端。
	win.forward(&ipc.Message{ID: "m9", Dir: ipc.DirRequest, Call: "Svc.Greet"})
	waitFor(t, func() bool { return len(win.delivered) == 1 })
	if got := string(win.delivered[0].Result); got != `"hello"` {
		t.Fatalf("结果 = %s", got)
	}

	// 关闭：remote 发 CtrlClose，shell 关窗，Run 返回。
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return win.closed })
	select {
	case <-runErr:
	case <-time.After(2 * time.Second):
		t.Fatal("remote.Run 未返回")
	}
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
