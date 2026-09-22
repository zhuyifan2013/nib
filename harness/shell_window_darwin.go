//go:build darwin

package harness

import (
	"net"
	"os"

	"nib.dev/nib/webview"
)

// serveShell 创建常驻窗口（DevTools 开，尺寸默认 1024x768）并进入事件循环。
func serveShell(ln net.Listener) error {
	title := os.Getenv("NIB_TITLE")
	if title == "" {
		title = "Nib Dev"
	}
	win := webview.New(webview.Options{Title: title, Width: 1024, Height: 768, DevTools: true})
	return NewShell(win, ln).Run()
}
