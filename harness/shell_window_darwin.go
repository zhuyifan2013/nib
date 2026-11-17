//go:build darwin

package harness

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"

	"nib.dev/nib/driver"
	"nib.dev/nib/webview"
)

// serveShell 创建常驻窗口（DevTools 开，尺寸默认 1024x768）并进入事件循环。
// eventsPath 非空时把窗口事件流以 NDJSON 导出："-" 为 stdout，否则为文件路径（追加）。
func serveShell(ln net.Listener, eventsPath string, opts ShellOptions) error {
	title := os.Getenv("NIB_TITLE")
	if title == "" {
		title = "Nib Dev"
	}
	win := webview.New(webview.Options{Title: title, Width: 1024, Height: 768, DevTools: true, Headless: opts.Headless})
	defer win.Close()
	done := make(chan struct{})
	defer close(done)
	if opts.Context != nil {
		go func() {
			select {
			case <-opts.Context.Done():
				win.Close()
			case <-done:
			}
		}()
	}
	if opts.Driver.Port != 0 {
		server, err := driver.Start(win, opts.Driver)
		if err != nil {
			return err
		}
		defer server.Close()
		log.Printf("nib driver: http://%s (Bearer authentication required)", server.Addr())
	}
	if opts.Notify != nil {
		opts.Notify("running", map[string]any{"driver_port": opts.Driver.Port})
	}
	if eventsPath != "" {
		defer exportEvents(win, eventsPath)()
	}
	shell := NewShell(win, ln)
	defer shell.Close()
	return shell.Run()
}

// exportEvents 订阅窗口事件总线并写 NDJSON（一行一条事件）。
func exportEvents(win *webview.Window, path string) func() {
	var w io.Writer
	var f *os.File
	if path == "-" {
		w = os.Stdout
	} else {
		var err error
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("harness: 事件导出 %s 打开失败: %v", path, err)
			return func() {}
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
	return sub.Unsubscribe
}
