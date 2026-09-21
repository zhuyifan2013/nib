---
id: m3-security
title: M3 安全层（权限注解 / 依赖审计 / 沙箱）
type: task
status: backlog
priority: medium
owner: ai
tags: [m3, security]
related: [m2-diagnostics]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# M3 安全层

绑定方法声明式权限注解 + 默认拒绝（生成绑定时的编译期检查）、构建时依赖审计、dev --sandbox 系统调用报告。前提：M2 事件流（安全层长在观测数据上）。设计依据见 `design-v0-1` 支柱三。
