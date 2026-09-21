---
id: dev-harness-complexity
title: Dev Harness 热替换技术难度
type: risk
status: open
probability: medium
impact: high
mitigation: 验收底线设为 <2s 快速重启，真热替换（goroutine 组替换 + 状态快照）作为理想形态逐步逼近
tags: [dev-experience, m1]
related: [dev-harness]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# Dev Harness 热替换难度

Go 无动态链接，进程内热替换业务代码没有现成机制。候选方案（业务层接口隔离 +
goroutine 组重启、go plugin、独立进程 + socket）各有代价。若攻坚失败，
降级为"常驻窗口 + 极速重启"仍能把 t_launch 压到远低于现状。
