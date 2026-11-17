---
id: mcp-server
title: M2 MCP 本机驱动接入
type: task
status: done
priority: high
owner: ai
tags: [m2, mcp, diagnostics]
related: [m2-diagnostics, local-driver, cli-structured-output]
blocked_by: []
created_at: 2026-11-17
updated_at: 2026-11-17
---

# MCP 本机驱动接入

提供经 stdio 启动的 MCP server，连接显式开启并使用 token 鉴权的本机驱动，暴露现有驱动操作，使 agent 可以验证界面和读取诊断结果。首版在 macOS 已登录 GUI session 中验收。

验收包括 MCP 客户端初始化与工具发现、鉴权、操作和错误传递、JSON-RPC stdout 纯净，以及 agent 使用真实应用完成界面操作、断言和事件读取；完成独立 review 与复核。

## 当前进展（2026-11-17）

使用官方 Go MCP SDK v1.0.0 实现独立的 `nib-mcp` stdio 进程，转发本机鉴权驱动的九个接口，使用方法见 `mcp-operations`。真实 macOS GUI 实测 MCP 初始化、绑定调用、click、fill、eval、ready、DOM、事件与截图通过。单测覆盖驱动请求和 SDK 客户端工具调用。`go test ./...` 与 `go test -race ./cmd/nib-mcp ./driver ./events ./harness` 通过。

独立 review 和修复复核通过，修复绑定参数中大整数精度损失及嵌套错误可能泄露 token 的问题。reviewer 独立执行 `go test -count=1 ./cmd/nib-mcp ./driver ./harness` 通过，修复后真实 GUI 操作闭环再次通过。驱动错误仅转发 HTTP 状态和错误码，具体错误正文需通过事件或本机诊断查看。
