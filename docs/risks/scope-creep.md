---
id: scope-creep
title: 里程碑前功能蔓延
type: risk
status: open
probability: high
impact: medium
mitigation: 设计文档明确的非目标清单（无 AI 运行时、无移动端、M3 安全层不做提前）作为验收闸门
tags: [process]
related: [design-v0-1]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 功能蔓延

框架题材天然诱人（插件系统、自动更新、移动端、AI 集成都想做）。
M1 未稳之前任何越里程碑开发都直接威胁核心指标。优先级以 tasks/ 中的
priority 字段为准，high 以下且属于 M2+ 的议题一律先记录为 backlog。
