---
id: local-driver
title: M2 本机 HTTP 驱动与隐藏窗口边界
type: decision
status: accepted
tags: [m2, driver, security]
related: [m2-diagnostics, cli-first, dev-harness-impl]
created_at: 2026-10-08
updated_at: 2026-10-08
---

# 本机驱动

驱动显式开启，默认 Port=0 不监听。macOS 直接 runtime 挂载原生窗口；dev 模式挂在常驻 shell，让业务进程热替换不影响端口。未选择在 Remote 业务进程挂载，因为截图与页面上下文实际由 shell 持有，且重启会切断驱动。

HTTP 绑定固定 127.0.0.1，所有请求（包括 readiness/事件）需要 Bearer token；token 最少 16 字节，通过 Options 或 NIB_DRIVER_TOKEN 提供，不放命令行、不记录 token。拒绝携带 Origin 的浏览器请求，不启用 CORS；这是开发者诊断入口，不是生产权限沙箱。

API 版本为 /v1。结果用 JSON 包裹（截图为 PNG），失败用 error.code/message 与 HTTP 状态。输入上限 1 MiB，禁止未知字段/多段 JSON；请求超时默认 5s、最大 30s；窗口操作单并发，忙时 429。eval 结果上限 4 MiB 字符、截图上限 32 MiB。事件读取轮询环形缓冲，返回 oldest_seq，调用方据此识别历史被覆盖。

EvalResult 接受 JS 表达式或脚本，以最后一个表达式的 JSON 可序列化值为结果，await Promise。click/fill 使用 CSS selector 与 DOM 事件。call 经过真实 window.nib.invoke → IPC → Go 路径，不绕过前端桥。DOM 快照当前是 HTML/title/url/readyState，不是 accessibility tree。navigate 只返回 accepted，页面加载完成需随后轮询 ready 和目标页面断言。

--headless 仅隐藏 NSWindow，仍需要已登录的 macOS GUI session/WindowServer；不宣称无显示器 CI。截图为 WKWebView 内容，尺寸/缩放由系统 backing scale 决定，不含窗口装饰。

超时终止 HTTP 等待和 Go pending 项；不能强制停止用户 JS 同步死循环或 Go handler。桥内 binding Promise 30s 超时清理自身 pending；已经启动的业务逻辑不因 HTTP 断开而撤销。原生 runtime 仍为单窗口、单事件循环，反复创建窗口的完整原生对象释放与真正取消属 R1/R2 后续工作。驱动关闭 listener/活动连接，shell 关闭 socket、业务会话和订阅；原生窗口关闭现在 stop AppKit run，让 defer 清理有机会运行。
