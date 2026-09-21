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
	"sync"
	"unsafe"

	"nib.dev/nib/ipc"
)

// registry 将 C 侧持有的 goHandle 映射回 Go 的 Window。
var registry sync.Map // unsafe.Pointer -> *Window

// Window 是 macOS 上 NSWindow + WKWebView 的封装。
type Window struct {
	handle   unsafe.Pointer // *C 结构（webview_darwin.m 中 malloc）
	goHandle unsafe.Pointer // 本对象指针，作为 registry 键
	handlers sync.Map       // name -> ipc.Handler
}

func New(opts Options) *Window {
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))
	h := C.nibWindowCreate(title, C.int(opts.Width), C.int(opts.Height), cbool(opts.DevTools))
	w := &Window{handle: h}
	w.goHandle = unsafe.Pointer(w)
	registry.Store(w.goHandle, w)
	C.nibWindowSetGoHandle(h, w.goHandle)
	return w
}

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
	v, ok := w.handlers.Load(msg.Call)
	if !ok {
		w.deliver(msg.ID, nil, ipc.NewError("ipc/not_found", "no binding for "+msg.Call))
		return
	}
	result, e := v.(ipc.Handler)(msg.Args)
	w.deliver(msg.ID, result, e)
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
	payload, err := ipc.Marshal(m)
	if err != nil {
		return
	}
	b64 := base64.StdEncoding.EncodeToString(payload)
	js := fmt.Sprintf(`window.__nibResolve(%q, %t, %q)`, id, ok, b64)
	w.Eval(js)
}

func (w *Window) Navigate(target string) error {
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
	C.nibWindowClose(w.handle)
	return nil
}

