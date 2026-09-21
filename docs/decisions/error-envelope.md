---
id: error-envelope
title: 跨边界错误信封设计
type: decision
status: accepted
tags: [ipc, diagnostics]
related: [go-backend, m2-diagnostics]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 跨边界错误信封

## 决定
单一错误格式贯穿 JS ↔ Go：{code, message, chain[], retryable, doc}。chain 把 JS 调用栈、IPC 记录、Go 栈帧缝合成一条因果链。Go 端 panic 一律转为结构化错误，不穿透到前端。

## 理由
AI 修复成功率取决于错误上下文的完整性；孤立字符串错误（Wails 现状）让 AI 无从定位。这是"框架对 AI 友好"的最小必要设计。

## 实现
ipc/protocol.go 中的 Error / Frame 类型。
