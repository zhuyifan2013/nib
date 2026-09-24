//go:build darwin

package webview

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>

// 由 webview_darwin.m 实现
extern void* nibWindowCreate(const char* title, int width, int height, int devtools);
extern void  nibWindowSetGoHandle(void* w, void* handle);
extern void  nibWindowNavigate(void* w, const char* target);
extern void  nibWindowEval(void* w, const char* js);
extern void  nibWindowSetTitle(void* w, const char* title);
extern void  nibWindowResize(void* w, int width, int height);
extern void  nibWindowRun(void* w);
extern void  nibWindowClose(void* w);
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"unsafe"

	"nib.dev/nib/events"
	"nib.dev/nib/ipc"
)

// registry 将 C 侧持有的 token（C 内存）映射回 Go 的 Window。
// 使用 C.malloc 分配的 token 而非 Go 指针，遵守 cgo 指针传递规则。
var registry sync.Map // unsafe.Pointer(C token) -> *Window

// Window 是 macOS 上 NSWindow + WKWebView 的封装。
type Window struct {
	handle   unsafe.Pointer         // *C 结构（webview_darwin.m 中 malloc）
	token    unsafe.Pointer         // C.malloc(1) 分配的注册表键
	handlers sync.Map               // name -> ipc.Handler
	forward  func(msg *ipc.Message) // 非空时 dispatch 整体转发（harness shell 用）
	events   *events.Bus            // 事件流（M2 诊断层）；构造时 Events() 可拿
}

func New(opts Options) *Window {
	// AppKit 要求主线程运行事件循环；锁定当前（主）goroutine 防止调度器迁移。
	runtime.LockOSThread()
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))
	h := C.nibWindowCreate(title, C.int(opts.Width), C.int(opts.Height), cbool(opts.DevTools))
	w := &Window{handle: h, token: C.malloc(1)}
	registry.Store(w.token, w)
	C.nibWindowSetGoHandle(h, w.token)
	w.events = opts.Events
	if w.events == nil {
		w.events = events.NewBus(0)
	}
	w.events.Emit(events.WindowCreated, map[string]any{
		"title": opts.Title, "width": opts.Width, "height": opts.Height, "devtools": opts.DevTools,
	})
	return w
}

// Events 返回窗口事件总线。
func (w *Window) Events() *events.Bus { return w.events }

func cbool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

func (w *Window) Bind(name string, h ipc.Handler) error {
	w.handlers.Store(name, h)
	return nil
}

//export nibIPCMessage
func nibIPCMessage(goHandle unsafe.Pointer, body *C.char) {
	defer C.free(unsafe.Pointer(body))
	v, ok := registry.Load(goHandle)
	if !ok {
		return
	}
	msg, err := ipc.Unmarshal([]byte(C.GoString(body)))
	if err != nil {
		return
	}
	v.(*Window).dispatch(msg)
}

func (w *Window) dispatch(msg *ipc.Message) {
	// 调用事件在转发判断之前发出：harness 模式下 shell 侧同样可见，
	// 并经事件流到达业务进程（驱动端口的观察数据源）。
	w.events.Emit(events.IPCCall, map[string]any{
		"id": msg.ID, "call": msg.Call, "args_bytes": len(msg.Args),
	})
	if w.forward != nil {
		w.forward(msg)
		return
	}
	v, ok := w.handlers.Load(msg.Call)
	if !ok {
		w.deliver(msg.ID, nil, ipc.NewError("ipc/not_found", "no binding for "+msg.Call))
		return
	}
	result, e := w.callHandler(msg, v.(ipc.Handler))
	w.deliver(msg.ID, result, e)
}

// callHandler 执行 handler 并兜底 panic：进程不崩，事件流记录 runtime.panic，
// 前端收到 ipc/panic 结构化错误（因果链由错误信封继续向上缝合）。
func (w *Window) callHandler(msg *ipc.Message, h ipc.Handler) (result any, e *ipc.Error) {
	defer func() {
		if r := recover(); r != nil {
			w.events.Emit(events.RuntimePanic, map[string]any{
				"call": msg.Call, "stack": string(debug.Stack()),
			})
			e = ipc.NewError("ipc/panic", fmt.Sprintf("handler %s panic: %v", msg.Call, r))
			result = nil
		}
	}()
	return h(msg.Args)
}

// SetForward 设置整体转发函数：非空时，前端调用不再分发给本地 handler，
// 而是交给 f（dev harness 的 shell 进程用它把调用转发给业务进程）。
func (w *Window) SetForward(f func(msg *ipc.Message)) {
	w.forward = f
}

// DeliverResponse 把一条已构造好的响应消息经 JS 桥送回前端（harness 转发用）。
func (w *Window) DeliverResponse(m *ipc.Message) {
	w.deliverMessage(m)
}

// deliver 把响应编码后经 JS 桥送回前端（base64 包裹避免转义问题）。
func (w *Window) deliver(id string, result any, e *ipc.Error) {
	m := &ipc.Message{ID: id, Dir: ipc.DirResponse}
	ok := e == nil
	if ok {
		raw, err := json.Marshal(result)
		if err != nil {
			m.Error = ipc.NewError("ipc/encode", err.Error())
			ok = false
		} else {
			m.Result = raw
		}
	} else {
		m.Error = e
	}
	w.emitDelivery(m, ok)
	payload, err := ipc.Marshal(m)
	if err != nil {
		return
	}
	w.deliverRaw(m.ID, ok, payload)
}

// deliverMessage 编码完整消息信封并送回前端。
func (w *Window) deliverMessage(m *ipc.Message) {
	w.emitDelivery(m, m.Dir == ipc.DirResponse && m.Error == nil)
	payload, err := ipc.Marshal(m)
	if err != nil {
		return
	}
	w.deliverRaw(m.ID, m.Dir == ipc.DirResponse && m.Error == nil, payload)
}

func (w *Window) deliverRaw(id string, ok bool, payload []byte) {
	b64 := base64.StdEncoding.EncodeToString(payload)
	js := fmt.Sprintf(`window.__nibResolve(%q, %t, %q)`, id, ok, b64)
	w.Eval(js)
}

// emitDelivery 把一次响应投递记录进事件流（结果与错误两条 kind）。
func (w *Window) emitDelivery(m *ipc.Message, ok bool) {
	if m.Error != nil {
		w.events.Emit(events.IPCError, map[string]any{
			"id": m.ID, "code": m.Error.Code, "message": m.Error.Message,
		})
	} else {
		data := map[string]any{"id": m.ID}
		if ok {
			data["result_bytes"] = len(m.Result)
		}
		w.events.Emit(events.IPCResponse, data)
	}
}

func (w *Window) Navigate(target string) error {
	w.events.Emit(events.WindowNavigated, map[string]any{"target": target})
	c := C.CString(target)
	defer C.free(unsafe.Pointer(c))
	C.nibWindowNavigate(w.handle, c)
	return nil
}

func (w *Window) Eval(js string) error {
	c := C.CString(js)
	defer C.free(unsafe.Pointer(c))
	C.nibWindowEval(w.handle, c)
	return nil
}

func (w *Window) SetTitle(title string) {
	c := C.CString(title)
	defer C.free(unsafe.Pointer(c))
	C.nibWindowSetTitle(w.handle, c)
}

func (w *Window) Resize(width, height int) {
	C.nibWindowResize(w.handle, C.int(width), C.int(height))
}

func (w *Window) Run() error {
	C.nibWindowRun(w.handle)
	return nil
}

func (w *Window) Close() error {
	w.events.Emit(events.WindowClosed, nil)
	C.nibWindowClose(w.handle)
	return nil
}
