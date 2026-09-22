package harness

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"nib.dev/nib/ipc"
)

// Remote 是 app 模式的 core.Window 实现：不建本地窗口，把绑定注册、
// 页面导航等控制消息发给 shell 进程，并把 shell 转发来的调用分发给本地 handler。
// 由 DialRemote 创建；业务源码无需改动——core.Run 检测到 NIB_SOCK 即启用。
type Remote struct {
	conn     net.Conn
	mu       sync.Mutex // 写锁：Send 可能并发（handler 调用 + 控制消息）
	handlers sync.Map   // name -> ipc.Handler

	pendingNav string // Run 之前的 Navigate 目标，随 CtrlInit 一起发送
	started    bool
	closed     chan struct{}
	closeOnce  sync.Once
}

// DialRemote 连接 shell 进程的 unix socket，返回远程窗口。
func DialRemote(sockPath string) (*Remote, error) {
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("连接 shell (%s): %w", sockPath, err)
	}
	return &Remote{conn: conn, closed: make(chan struct{})}, nil
}

func (r *Remote) send(m *ipc.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Send(r.conn, m)
}

// Bind 注册本地 handler（Run 时随 CtrlInit 把绑定名上报 shell）。
func (r *Remote) Bind(name string, h ipc.Handler) error {
	r.handlers.Store(name, h)
	return nil
}

// Navigate 记录起始页面；Run 之前调用则随 CtrlInit 发送，之后调用走 CtrlNavigate。
func (r *Remote) Navigate(target string) error {
	if r.started {
		return r.send(&ipc.Message{
			ID:   "ctrl",
			Dir:  ipc.DirRequest,
			Call: CtrlNavigate,
			Args: marshalArgs(target),
		})
	}
	r.pendingNav = target
	return nil
}

// Eval 在 shell 的 WebView 中执行 JS（不取回结果）。
func (r *Remote) Eval(js string) error {
	return r.send(&ipc.Message{ID: "ctrl", Dir: ipc.DirRequest, Call: CtrlEval, Args: marshalArgs(js)})
}

func (r *Remote) SetTitle(title string) {
	r.send(&ipc.Message{ID: "ctrl", Dir: ipc.DirRequest, Call: CtrlSetTitle, Args: marshalArgs(title)})
}

func (r *Remote) Resize(width, height int) {
	r.send(&ipc.Message{
		ID:   "ctrl",
		Dir:  ipc.DirRequest,
		Call: CtrlResize,
		Args: marshalArgs(struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}{width, height}),
	})
}

// Run 上报绑定与起始页面，然后阻塞服务 shell 转发来的调用，直到连接关闭。
func (r *Remote) Run() error {
	r.started = true
	var bindings []string
	r.handlers.Range(func(k, _ any) bool {
		bindings = append(bindings, k.(string))
		return true
	})
	init := initPayload{Navigate: r.pendingNav, Bindings: bindings}
	if err := r.send(&ipc.Message{ID: "init", Dir: ipc.DirRequest, Call: CtrlInit, Args: marshalArgs(init)}); err != nil {
		return err
	}
	return serveConn(r.conn, func(m *ipc.Message) {
		if m.Dir != ipc.DirRequest || IsCtrl(m.Call) {
			return // 控制响应无需处理
		}
		r.dispatch(m)
	})
}

// dispatch 在独立 goroutine 中执行 handler，避免慢方法阻塞后续消息。
func (r *Remote) dispatch(m *ipc.Message) {
	go func() {
		resp := &ipc.Message{ID: m.ID, Dir: ipc.DirResponse}
		v, ok := r.handlers.Load(m.Call)
		if !ok {
			resp.Error = ipc.NewError("ipc/not_found", "no binding for "+m.Call)
		} else {
			result, e := v.(ipc.Handler)(m.Args)
			if e != nil {
				resp.Error = e
			} else if raw, err := json.Marshal(result); err != nil {
				resp.Error = ipc.NewError("ipc/encode", err.Error())
			} else {
				resp.Result = raw
			}
		}
		r.send(resp)
	}()
}

// Close 通知 shell 关闭窗口并断开连接。
func (r *Remote) Close() error {
	var err error
	r.closeOnce.Do(func() {
		err = r.send(&ipc.Message{ID: "ctrl", Dir: ipc.DirRequest, Call: CtrlClose, Args: marshalArgs(nil)})
		r.conn.Close()
		close(r.closed)
	})
	return err
}
