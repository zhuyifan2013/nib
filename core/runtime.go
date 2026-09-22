// Package core 定义 Nib 运行时平台无关的核心抽象。
// 平台相关实现位于 webview/ 包（按构建标签拆分），此处仅持有接口。
package core

import (
	"os"

	"nib.dev/nib/harness"
	"nib.dev/nib/ipc"
)

// Handler 是绑定方法的签名（别名自 ipc，避免 core ↔ webview 循环依赖）。
type Handler = ipc.Handler

// Options 是创建窗口的参数。
type Options struct {
	Title  string
	Width  int
	Height int

	// DevTools 是否允许打开开发者工具（开发模式）。
	DevTools bool
}

// Window 是平台窗口 + WebView 的抽象。
type Window interface {
	// Navigate 加载 URL（http(s):// / nib:// / file:// file://），或以 "<" 开头的内嵌 HTML 字符串。
	Navigate(target string) error

	// Eval 在 WebView 上下文中执行 JS，不取回结果。
	Eval(js string) error

	// Bind 应用一个服务方法绑定（由 Runtime.Run 在窗口创建后统一调用）。
	Bind(name string, h Handler) error

	SetTitle(title string)
	Resize(width, height int)

	// Run 进入平台事件循环，阻塞至窗口关闭。
	Run() error

	// Close 关闭窗口并退出事件循环。
	Close() error
}

// Runtime 是应用运行时入口。绑定在 Run 之前注册，窗口创建时统一应用。
type Runtime struct {
	opts     Options
	bindings map[string]Handler
}

func New(opts Options) *Runtime {
	if opts.Width == 0 {
		opts.Width = 1024
	}
	if opts.Height == 0 {
		opts.Height = 768
	}
	return &Runtime{opts: opts, bindings: map[string]Handler{}}
}

// openWindow 选择窗口实现：dev harness 远程窗口或平台原生窗口。
func (r *Runtime) openWindow() (Window, error) {
	if sock := os.Getenv("NIB_SOCK"); sock != "" {
		return harness.DialRemote(sock)
	}
	return newWindow(r.opts) // 由平台文件（runtime_$(GOOS).go）提供
}

// Bind 注册一个可被前端通过 window.nib.invoke("Service.Method", args) 调用的方法。
func (r *Runtime) Bind(name string, h Handler) {
	r.bindings[name] = h
}

// Run 创建主窗口、应用全部绑定并进入事件循环。
// 环境变量 NIB_SOCK 存在时（dev harness 模式），不建本地窗口，
// 改为连接常驻 shell 进程：业务代码同一份，运行形态由环境切换。
func (r *Runtime) Run(startURL string) error {
	win, err := r.openWindow()
	if err != nil {
		return err
	}
	for name, h := range r.bindings {
		if err := win.Bind(name, h); err != nil {
			return err
		}
	}
	if err := win.Navigate(startURL); err != nil {
		return err
	}
	return win.Run()
}
