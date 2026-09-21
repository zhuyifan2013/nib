---
id: go-backend
title: 后端语言选 Go
type: decision
status: accepted
tags: [language, architecture]
related: [system-webview, no-ai-runtime]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 后端语言选 Go

## 背景
候选：Rust（Tauri）、Go（Wails 已验证）、C++、C#、Zig。

## 决定
Go。

## 理由
- 编译速度是北极星指标 T_iter 的第一变量：Go 增量编译 ~秒级，Rust 首构 ~340s，对 AI 迭代回路是质变
- 开发者池远大于 Rust（JS+Go 组合大众），单一语言全栈降低 AI 跨语言上下文错误率
- 标准库强、静态链接单二进制天然契合"小体积壳"目标
- 代价明确接受：GC 延迟峰值（毫秒级，桌面应用无感）、二进制比 Rust 大 2-3MB
