---
id: demo-e2e-manual
title: Demo 全链路人工验证（macOS）
type: test
status: passing
kind: manual
tags: [m1, e2e]
related: [minimal-runtime-mac]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# Demo 全链路人工验证

## 步骤
`go run ./cmd/demo` → 点击按钮 → 断言显示 "Hello, AI! (from Go)"。

## 结果（2026-09-21）
通过。链路：窗口 → JS 桥（window.nib.invoke）→ IPC 消息信封 → Go 绑定分发 → base64 响应 → Promise resolve。

## 过程中发现并修复的缺陷
1. cgo 指针规则：Go 指针存入 C 结构 panic → 改 C.malloc token 注册表
2. 桥接未解包信封 → 前端收到 [object Object] → resolve(env.result) / reject(env.error)
