package harness

import (
	"encoding/json"
	"log"
	"net"
	"sync"

	"nib.dev/nib/events"
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
	Events() *events.Bus // M2 事件流：窗口/IPC 事件的产生地
}

// Shell 是常驻层：窗口 + WebView + 业务进程 socket 转发。
// 业务进程重启期间，前端调用收到可重试的 harness/app_unavailable。
//
// 事件流（M2）：窗口事件总线的订阅者有两类——
//   - 当前连接的业务进程（CtrlEvents 消息，重连后先重放缓冲，保证业务进程持有完整流）
//   - shell 自己的导出器（RunShell 的 --events，NDJSON）
type Shell struct {
	win shellWindow
	ln  net.Listener

	mu      sync.Mutex
	app     *Session // 当前业务进程会话（Decoder 常驻，Send 并发安全）
	appName string   // 最近一次 init 上报的绑定数等，仅日志用
	online  bool

	eventMu  sync.Mutex
	recent   []events.Event // 待重放缓冲（上限 replayBufferCap）
	eventSub *events.Subscription
}

// replayBufferCap 是 shell 为重连业务进程保留的事件数（热替换窗口内的观测不丢）。
const replayBufferCap = 512

// NewShell 创建常驻层；win 的所有前端调用会被转发给当前连接的业务进程。
func NewShell(win shellWindow, ln net.Listener) *Shell {
	s := &Shell{win: win, ln: ln}
	win.SetForward(s.forward)
	// Subscribe(replay)：窗口创建等早期事件已在总线环形缓冲中，重放进 pump，
	// 保证首个业务进程也能拿到完整前缀。
	s.eventSub = win.Events().Subscribe(true)
	go s.pumpEvents()
	return s
}

// pumpEvents 把窗口事件写入重放缓冲并推给当前业务进程。
// 推送失败静默丢弃：业务进程断线由 app_disconnected 事件标记，不重试。
func (s *Shell) pumpEvents() {
	for ev := range s.eventSub.C {
		s.eventMu.Lock()
		s.recent = append(s.recent, ev)
		if len(s.recent) > replayBufferCap {
			s.recent = s.recent[len(s.recent)-replayBufferCap:]
		}
		s.eventMu.Unlock()

		s.mu.Lock()
		app, online := s.app, s.online
		s.mu.Unlock()
		if online {
			raw, err := json.Marshal([]events.Event{ev})
			if err == nil {
				app.Send(&ipc.Message{ID: "evt", Dir: ipc.DirEvent, Call: CtrlEvents, Args: raw})
			}
		}
	}
}

// replayEvents 把缓冲中的事件批量发给（重连的）业务进程。
func (s *Shell) replayEvents(sess *Session) {
	s.eventMu.Lock()
	batch := make([]events.Event, len(s.recent))
	copy(batch, s.recent)
	s.eventMu.Unlock()
	if len(batch) == 0 {
		return
	}
	if raw, err := json.Marshal(batch); err == nil {
		sess.Send(&ipc.Message{ID: "evt", Dir: ipc.DirEvent, Call: CtrlEvents, Args: raw})
	}
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
	sess := NewSession(conn)
	m, err := sess.Recv()
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
		s.app.Conn().Close() // 旧进程还在：抢占（正常重启是先断后连，这里是保险）
	}
	s.app = sess
	s.online = true
	s.mu.Unlock()

	log.Printf("harness: 业务进程已连接，绑定 %d 个方法", len(init.Bindings))
	s.win.Events().Emit(events.AppConnected, map[string]any{"bindings": len(init.Bindings)})
	s.replayEvents(sess)
	if init.Navigate != "" {
		if err := s.win.Navigate(init.Navigate); err != nil {
			log.Printf("harness: navigate 失败: %v", err)
		}
	}

	serveConn(sess, func(m *ipc.Message) {
		switch {
		case m.Dir == ipc.DirResponse:
			s.win.DeliverResponse(m)
		case m.Dir == ipc.DirRequest && IsCtrl(m.Call):
			s.handleCtrl(m) // 业务进程主动控制：close / navigate / eval 等
		}
	})

	s.mu.Lock()
	if s.app != nil && s.app.Conn() == conn {
		s.app = nil
		s.online = false
	}
	s.mu.Unlock()
	s.win.Events().Emit(events.AppDisconnected, nil)
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
	if err := app.Send(m); err != nil {
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
	s.eventSub.Unsubscribe()
	s.ln.Close()
	return s.win.Close()
}
