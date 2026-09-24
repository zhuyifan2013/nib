package harness

import (
	"encoding/json"
	"fmt"
	"net"
	"runtime/debug"
	"sync"

	"nib.dev/nib/events"
	"nib.dev/nib/ipc"
)

// Remote 是 app 模式的 core.Window 实现：不建本地窗口，把绑定注册、
// 页面导航等控制消息发给 shell 进程，并把 shell 转发来的调用分发给本地 handler。
// 由 DialRemote 创建；业务源码无需改动——core.Run 检测到 NIB_SOCK 即启用。
//
// 事件流方向：shell 侧产生窗口/IPC 事件，经 CtrlEvents 消息推给 Remote，
// Remote 汇入 events 总线——业务进程因此持有完整事件流（驱动端口的挂载点，M2 增量 2）。
type Remote struct {
	sess     *Session // 与 shell 的会话（Decoder 常驻；Send 经 wmu 并发安全）
	handlers sync.Map // name -> ipc.Handler
	events   *events.Bus

	pendingNav string // Run 之前的 Navigate 目标，随 CtrlInit 一起发送
	started    bool
	closed     chan struct{}
	closeOnce  sync.Once
}

// DialRemote 连接 shell 进程的 unix socket，返回远程窗口。
// bus 为 nil 时内部自建；core 传入 Runtime 的总线使事件流对应用可见。
func DialRemote(sockPath string, bus *events.Bus) (*Remote, error) {
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("连接 shell (%s): %w", sockPath, err)
	}
	if bus == nil {
		bus = events.NewBus(0)
	}
	return &Remote{sess: NewSession(conn), events: bus, closed: make(chan struct{})}, nil
}

// Events 返回事件总线。
func (r *Remote) Events() *events.Bus { return r.events }

func (r *Remote) send(m *ipc.Message) error {
	return r.sess.Send(m)
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
	return serveConn(r.sess, func(m *ipc.Message) {
		if m.Call == CtrlEvents {
			var evs []events.Event
			if err := json.Unmarshal(m.Args, &evs); err == nil {
				for _, ev := range evs {
					r.events.Inject(ev)
				}
			}
			return
		}
		if m.Dir != ipc.DirRequest || IsCtrl(m.Call) {
			return // 控制响应无需处理
		}
		r.dispatch(m)
	})
}

// dispatch 在独立 goroutine 中执行 handler，避免慢方法阻塞后续消息。
// handler panic 被兜底：进程不崩，事件流记录 runtime.panic，前端收到 ipc/panic。
func (r *Remote) dispatch(m *ipc.Message) {
	go func() {
		resp := &ipc.Message{ID: m.ID, Dir: ipc.DirResponse}
		v, ok := r.handlers.Load(m.Call)
		if !ok {
			resp.Error = ipc.NewError("ipc/not_found", "no binding for "+m.Call)
		} else {
			result, e := callRecovered(r.events, m.Call, v.(ipc.Handler), m.Args)
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

// callRecovered 执行 handler 并 recover panic：错误信封 + runtime.panic 事件。
func callRecovered(bus *events.Bus, call string, h ipc.Handler, args json.RawMessage) (result any, e *ipc.Error) {
	defer func() {
		if rec := recover(); rec != nil {
			bus.Emit(events.RuntimePanic, map[string]any{
				"call": call, "stack": string(debug.Stack()),
			})
			e = ipc.NewError("ipc/panic", fmt.Sprintf("handler %s panic: %v", call, rec))
			result = nil
		}
	}()
	return h(args)
}

// Close 通知 shell 关闭窗口并断开连接。
func (r *Remote) Close() error {
	var err error
	r.closeOnce.Do(func() {
		err = r.send(&ipc.Message{ID: "ctrl", Dir: ipc.DirRequest, Call: CtrlClose, Args: marshalArgs(nil)})
		r.sess.Conn().Close()
		close(r.closed)
	})
	return err
}
