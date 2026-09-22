package harness

import (
	"log"
	"net"
	"sync"

	"nib.dev/nib/ipc"
)

// shellWindow 是 shell 进程对窗口的最小需求（*webview.Window 实现；测试用 fake）。
type shellWindow interface {
	SetForward(func(*ipc.Message))
	DeliverResponse(*ipc.Message)
	Navigate(target string) error
	Eval(js string) error
	SetTitle(title string)
	Resize(width, height int)
	Run() error
	Close() error
}

// Shell 是常驻层：窗口 + WebView + 业务进程 socket 转发。
// 业务进程重启期间，前端调用收到可重试的 harness/app_unavailable。
type Shell struct {
	win shellWindow
	ln  net.Listener

	mu      sync.Mutex
	app     net.Conn
	appName string // 最近一次 init 上报的绑定数等，仅日志用
	online  bool
}

// NewShell 创建常驻层；win 的所有前端调用会被转发给当前连接的业务进程。
func NewShell(win shellWindow, ln net.Listener) *Shell {
	s := &Shell{win: win, ln: ln}
	win.SetForward(s.forward)
	return s
}

// Run 接受业务进程连接并进入窗口事件循环；窗口关闭时返回。
// accept 循环跑在独立 goroutine，返回 win.Run() 的结果。
func (s *Shell) Run() error {
	go s.acceptLoop()
	return s.win.Run()
}

func (s *Shell) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener 已关闭
		}
		go s.serveApp(conn)
	}
}

func (s *Shell) serveApp(conn net.Conn) {
	m, err := Recv(conn)
	if err != nil || m.Call != CtrlInit {
		log.Printf("harness: 业务进程首消息非法，断开")
		conn.Close()
		return
	}
	var init initPayload
	if err := unmarshalArgs(m.Args, &init); err != nil {
		log.Printf("harness: init 参数解析失败: %v", err)
		conn.Close()
		return
	}

	s.mu.Lock()
	if s.app != nil {
		s.app.Close() // 旧进程还在：抢占（正常重启是先断后连，这里是保险）
	}
	s.app = conn
	s.online = true
	s.mu.Unlock()

	log.Printf("harness: 业务进程已连接，绑定 %d 个方法", len(init.Bindings))
	if init.Navigate != "" {
		if err := s.win.Navigate(init.Navigate); err != nil {
			log.Printf("harness: navigate 失败: %v", err)
		}
	}

	serveConn(conn, func(m *ipc.Message) {
		switch {
		case m.Dir == ipc.DirResponse:
			s.win.DeliverResponse(m)
		case m.Dir == ipc.DirRequest && IsCtrl(m.Call):
			s.handleCtrl(m) // 业务进程主动控制：close / navigate / eval 等
		}
	})

	s.mu.Lock()
	if s.app == conn {
		s.app = nil
		s.online = false
	}
	s.mu.Unlock()
	log.Printf("harness: 业务进程已断开")
}

// forward 是前端调用的入口：控制消息本地处理，其余转发给业务进程。
func (s *Shell) forward(m *ipc.Message) {
	if IsCtrl(m.Call) {
		s.handleCtrl(m)
		return
	}
	s.mu.Lock()
	app, online := s.app, s.online
	s.mu.Unlock()
	if !online {
		s.win.DeliverResponse(&ipc.Message{
			ID:    m.ID,
			Dir:   ipc.DirResponse,
			Error: &ipc.Error{Code: errAppDown, Message: "业务进程不在线（热替换中？）", Retryable: true},
		})
		return
	}
	if err := Send(app, m); err != nil {
		s.win.DeliverResponse(&ipc.Message{
			ID:    m.ID,
			Dir:   ipc.DirResponse,
			Error: ipc.NewError(errBadTransport, err.Error()),
		})
	}
}

func (s *Shell) handleCtrl(m *ipc.Message) {
	switch m.Call {
	case CtrlClose:
		s.win.Close()
	case CtrlNavigate:
		var target string
		if err := unmarshalArgs(m.Args, &target); err == nil {
			s.win.Navigate(target)
		}
	case CtrlSetTitle:
		var title string
		if err := unmarshalArgs(m.Args, &title); err == nil {
			s.win.SetTitle(title)
		}
	case CtrlResize:
		var size struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}
		if err := unmarshalArgs(m.Args, &size); err == nil {
			s.win.Resize(size.Width, size.Height)
		}
	case CtrlEval:
		var js string
		if err := unmarshalArgs(m.Args, &js); err == nil {
			s.win.Eval(js)
		}
	}
}

// Close 关闭 listener 与窗口。
func (s *Shell) Close() error {
	s.ln.Close()
	return s.win.Close()
}
