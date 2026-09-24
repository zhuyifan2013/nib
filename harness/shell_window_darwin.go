//go:build darwin

package harness

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"

	"nib.dev/nib/webview"
)

// serveShell 创建常驻窗口（DevTools 开，尺寸默认 1024x768）并进入事件循环。
// eventsPath 非空时把窗口事件流以 NDJSON 导出："-" 为 stdout，否则为文件路径（追加）。
func serveShell(ln net.Listener, eventsPath string) error {
	title := os.Getenv("NIB_TITLE")
	if title == "" {
		title = "Nib Dev"
	}
	win := webview.New(webview.Options{Title: title, Width: 1024, Height: 768, DevTools: true})
	if eventsPath != "" {
		exportEvents(win, eventsPath)
	}
	return NewShell(win, ln).Run()
}

// exportEvents 订阅窗口事件总线并写 NDJSON（一行一条事件）。
func exportEvents(win *webview.Window, path string) {
	var w io.Writer
	var f *os.File
	if path == "-" {
		w = os.Stdout
	} else {
		var err error
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("harness: 事件导出 %s 打开失败: %v", path, err)
			return
		}
		w = f
	}
	sub := win.Events().Subscribe(true)
	go func() {
		defer sub.Unsubscribe()
		if f != nil {
			defer f.Close()
		}
		enc := json.NewEncoder(w)
		for ev := range sub.C {
			if err := enc.Encode(ev); err != nil {
				return // 管道断开等：导出器安静退出，不影响运行时
			}
		}
	}()
}
