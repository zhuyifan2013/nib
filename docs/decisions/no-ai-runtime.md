---
id: no-ai-runtime
title: 框架不内置 AI 运行时
type: decision
status: accepted
tags: [scope, ai]
related: [go-backend]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 不内置 AI 运行时

## 决定
不做 llm 抽象、模型管理、embedding、function calling 打通。

## 理由
- 这层变化太快、不是框架职责，做不过专门厂商
- 违反"极小 API 表面积"原则
- 框架对 AI 时代的价值在迭代回路与机器可操作性，不在 AI 功能本身
- 框架只保证"接 AI 能力的通道好用"：stream 类绑定（token 流）、绑定生成器
