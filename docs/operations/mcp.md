---
id: mcp-operations
title: MCP 驱动使用
type: doc
status: active
owner: ai
tags: [mcp, diagnostics]
related: [mcp-server, driver]
created_at: 2026-11-17
updated_at: 2026-11-17
---

# MCP 驱动使用

先在已登录的 macOS GUI 会话中启动启用驱动的应用，使用随机的 `NIB_DRIVER_TOKEN`，例如按 `driver` 操作文档启动 `nib dev --headless --driver-port 8123`。在 MCP 客户端中启动 `nib-mcp --driver-port 8123`，向子进程传入同一个 `NIB_DRIVER_TOKEN`。可先用 `go build -o nib-mcp ./cmd/nib-mcp` 构建该程序。端口必须为 1 到 65535，连接地址固定为 127.0.0.1。

MCP 使用 stdio，stdout 保留给协议，生命周期由客户端管理。工具包括 `ready`、`events`、`navigate`、`eval`、`call`、`dom`、`click`、`fill` 和 `screenshot`，对应 `driver` 的接口。`screenshot` 返回 MCP PNG 图片内容，其他结果包含驱动 JSON。驱动失败返回 MCP 工具错误，包含 HTTP 状态和驱动错误码；为避免错误详情暴露 token，任意错误正文不经此接口转发，可通过本机驱动诊断和事件查看详情。`ready` 只表示页面和桥就绪，业务绑定可能仍在连接中。

驱动默认关闭，服务需要显式开启并使用相同 token。隐藏窗口需要 GUI session，当前验证范围为 macOS。
