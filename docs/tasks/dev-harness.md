---
id: dev-harness
title: Dev Harness 常驻外壳（后端热替换）
type: task
status: done
priority: high
owner: ai
tags: [m1, dev-experience]
related: [dev-harness-complexity, binding-generator]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-22
---

# Dev Harness 常驻外壳

## 目标
开发态两层结构：常驻层（窗口 + WebView + 前端 HMR）永不退出；可替换层（用户 Go 业务代码）改动后 <1s 热替换，窗口无闪烁、状态可选保留。把 T_iter 的 t_launch 压到 ≈0（当前改 Go 代码需重启整个应用）。

## 约束
Go 无动态链接，真热替换难度高，见风险 `dev-harness-complexity`。验收底线：总耗时 <2s；理想形态：业务 goroutine 组重启 + 状态快照恢复。

## 结果与验证

**Result:** verified — 两进程 socket 转发方案落地，实现与取舍见 `dev-harness-impl`（decision）。
`nib dev ./cmd/demo`：窗口常驻，改业务代码 ≈1.1s 热替换（<2s 底线），构建失败保留旧进程，
业务离线期间前端收到可重试的 `harness/app_unavailable`。业务源码零改动（NIB_SOCK 环境切换）。

- 常驻层：`nib shell`（内部）— 窗口 + WebView + socket 转发
- 可替换层：同一份业务二进制，NIB_SOCK 存在时 core 自动切远程窗口
- 业务状态不保留（进程级替换）；前端窗口/JS 状态保留
- 实测记录见 `dev-harness-e2e`（test）；单测 `go test ./harness`
