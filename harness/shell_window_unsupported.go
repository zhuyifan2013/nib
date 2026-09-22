//go:build !darwin

package harness

import (
	"fmt"
	"net"
	"runtime"
)

// serveShell 在非 darwin 平台上暂未实现（见 webview-windows / webview-linux 任务）。
func serveShell(ln net.Listener) error {
	_ = ln.Close()
	return fmt.Errorf("dev harness shell 暂不支持 %s", runtime.GOOS)
}
