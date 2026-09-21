---
id: minimal-runtime-mac
title: M1 最小运行时（macOS）
type: task
status: done
priority: high
owner: ai
tags: [m1, runtime, macos]
related: [demo-e2e-manual, system-webview, go-backend]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# M1 最小运行时（macOS）

## 范围
IPC 协议骨架（错误信封 + 消息信封）、core 平台抽象、macOS WKWebView CGO 绑定、demo 应用。

## 结果与验证

**Result:** verified — `go run ./cmd/demo` 全链路人工验证通过，记录见 `demo-e2e-manual`；编译 1.4s / 3.8MB。
`go run ./cmd/demo` 全链路验证通过：窗口 → JS 桥（window.nib.invoke）→ IPC 消息信封 → Go 绑定分发 → base64 响应 → Promise 返回干净结果。验证记录见 `demo-e2e-manual`。增量编译 1.4s，二进制 3.8MB。

## 遗留
Windows（WebView2）与 Linux（WebKitGTK）绑定未实现，见 `webview-windows` / `webview-linux`。
