---
id: nib
title: Nib — AI 时代跨平台桌面框架
type: project
status: active
health: green
owner: ai
tags: [desktop, framework, go, webview]
created_at: 2026-09-21
updated_at: 2026-10-08
---

# Nib — AI 时代跨平台桌面框架

系统 WebView 壳 + Go 核心的轻量跨平台桌面框架。设计文档与完整决策见 `design-v0-1`。

## 北极星指标

AI 单次迭代时间 **T_iter < 5s** = 编译(<1.5s) + 启动(dev≈0) + 观察(<1s) + 上下文(一次到位)。

## 当前焦点

M1 macOS 运行时、绑定生成器与开发外壳已验证；M2 事件流增量已完成，
下一步为无头驱动。Windows/Linux 仍待实现。

正式发布目标：交付开发者与 agent 均可完成创建、开发、验证、构建、发行、更新闭环的
完整桌面框架。发布工作包、依赖顺序和验收见 `release-v1`。

## 实测基线（2026-09-21, macOS arm64）

| 指标 | 实测 | 目标 |
|------|------|------|
| 增量编译 | 1.4s | <1.5s ✅ |
| 二进制体积 | 3.8MB | <10MB ✅ |
