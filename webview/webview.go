// Package webview 封装各操作系统的系统 WebView。
// 平台实现按构建标签拆分：webview_darwin.go / webview_windows.go / webview_linux.go
package webview

// Options 是创建窗口的参数（与 core.Options 字段保持一致，结构满足避免循环依赖）。
type Options struct {
	Title    string
	Width    int
	Height   int
	DevTools bool
}
