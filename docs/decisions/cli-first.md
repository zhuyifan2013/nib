---
id: cli-first
title: CLI 优先：一切能力必须有机器可读的 CLI 入口
type: decision
status: accepted
tags: [architecture, agent-first, cli]
related: [no-ai-runtime, frontend-stack, m2-diagnostics, demo-apps]
created_at: 2026-09-22
updated_at: 2026-09-22
---

# CLI 优先：一切能力必须有机器可读的 CLI 入口

## 决定
Nib 的每一项框架能力都必须同时提供 CLI / 机器可读接口（结构化输出，如 `--json`），CLI 是能力的完整入口而非 GUI 的附属品。AI agent 是 CLI 的一等用户；人类通过同一入口（CLI 或 GUI）获得相同能力。

## 理由
- AI agent 的主场是进程 + 管道 + 结构化输出，不是像素。能力只有进了 CLI，才在 agent 的可达范围内。
- 与 `no-ai-runtime` 互补：框架不内置 AI，但保证"接 AI 的通道"——CLI 就是那条通道的物理形态。
- 可测试性副产品：CLI 可断言 ⇒ GUI 行为可回归（demo-apps 的 agent-native 验收标准建立于此）。
- GUI 与 CLI 是同一能力的两个渲染端，避免"GUI 功能 CLI 残缺"造成的二等公民能力面。

## 约束
- 新能力评审的第一问：它的 CLI 入口和 `--json` 输出长什么样？
- 输出必须结构化、稳定（schema 变更视为 breaking change）。
- GUI 专属的交互（拖拽、视觉反馈）不受此约束，但其底层状态变更仍需可经 CLI 达到。

## 验证方式
M2 诊断层（任务 `m2-diagnostics`）：CLI 全命令 `--json`、事件流、无头驱动（`--headless --driver-port`）是此决策的首次全面实施；demo-apps 的验收标准是其终验。
