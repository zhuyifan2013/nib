// Package core 定义 Nib 运行时平台无关的核心抽象。
// 平台相关实现位于 webview/ 包（按构建标签拆分），此处仅持有接口。
package core

import (
	"fmt"
	"nib.dev/nib/driver"
	"nib.dev/nib/events"

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
	Headless bool
	Driver   driver.Options // Explicit opt-in; zero Port disables automation.
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
	eventBus *events.Bus // M2 事件流；本地模式窗口事件汇入此总线，harness 模式下由 shell 转发汇入
}

func New(opts Options) *Runtime {
	if opts.Width == 0 {
		opts.Width = 1024
	}
	if opts.Height == 0 {
		opts.Height = 768
	}
	return &Runtime{opts: opts, bindings: map[string]Handler{}, eventBus: events.NewBus(0)}
}

// Events 返回运行时事件总线（窗口/IPC/harness 生命周期事件）。
// 订阅它即可实现结构化观察；harness 模式下业务进程经此拿到 shell 转发的完整流。
func (r *Runtime) Events() *events.Bus { return r.eventBus }

// openWindow 选择窗口实现：dev harness 远程窗口或平台原生窗口。
func (r *Runtime) openWindow() (Window, error) {
	if sock := os.Getenv("NIB_SOCK"); sock != "" {
		// harness 模式：事件由 shell 侧产生，经 ctrl 消息转发汇入本总线。
		return harness.DialRemote(sock, r.eventBus)
	}
	return newWindow(r.opts, r.eventBus) // 由平台文件（runtime_$(GOOS).go）提供
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
	defer win.Close()
	if r.opts.Driver.Port != 0 {
		native, ok := win.(driver.Window)
		if !ok {
			return fmt.Errorf("driver unsupported on this window; use nib dev driver flags for harness mode")
		}
		server, err := driver.Start(native, r.opts.Driver)
		if err != nil {
			return err
		}
		defer server.Close()
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
