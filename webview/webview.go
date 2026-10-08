// Package webview 封装各操作系统的系统 WebView。
// 平台实现按构建标签拆分：webview_darwin.go / webview_windows.go / webview_linux.go
package webview

import "nib.dev/nib/events"

// Options 是创建窗口的参数（与 core.Options 字段保持一致，结构满足避免循环依赖）。
type Options struct {
	Title    string
	Width    int
	Height   int
	DevTools bool
	Headless bool // Hide the window; a macOS GUI session is still required.

	// Events 是窗口事件的总线；nil 时内部自建（Window.Events() 始终可用）。
	// core 传入自己的总线，使窗口事件与运行时事件同属一条流。
	Events *events.Bus
}
