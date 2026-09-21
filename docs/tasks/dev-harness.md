---
id: dev-harness
title: Dev Harness 常驻外壳（后端热替换）
type: task
status: todo
priority: high
owner: ai
tags: [m1, dev-experience]
related: [dev-harness-complexity, binding-generator]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# Dev Harness 常驻外壳

## 目标
开发态两层结构：常驻层（窗口 + WebView + 前端 HMR）永不退出；可替换层（用户 Go 业务代码）改动后 <1s 热替换，窗口无闪烁、状态可选保留。把 T_iter 的 t_launch 压到 ≈0（当前改 Go 代码需重启整个应用）。

## 约束
Go 无动态链接，真热替换难度高，见风险 `dev-harness-complexity`。验收底线：总耗时 <2s；理想形态：业务 goroutine 组重启 + 状态快照恢复。
