---
id: demo-apps
title: 生产价值示范性项目（agent-native dogfood）
type: task
status: backlog
priority: medium
owner: ai
tags: [demo, frontend, agent-first]
related: [frontend-stack, m2-diagnostics]
blocked_by: []
created_at: 2026-09-22
updated_at: 2026-09-22
---

# 生产价值示范性项目

构建若干有生产价值的示范性应用，证明 Nib 的 agent-first 主张：**agent 零手动交互即可交付一个桌面应用**。

## 核心验收标准（agent-native，写死）

每个示范应用必须满足：
- 开发与验证全程由 agent 驱动：写代码 → 无头启动（M2）→ 经事件流/桥驱动 UI → 断言状态
- 唯一允许的人工环节是最终的视觉确认（截图）；迭代闭环内不依赖看屏、点鼠标
- UI 回归测试可在无显示器 CI 中运行（依赖 M2 无头驱动 + 事件流）

这条标准是框架级主张的演示：app = API，UI 表面可被机器完整读写——这是相对 Electron/Tauri 的主动差异化，不只是"AI 恰好熟悉 Web 栈"的被动优势。

## 首选 demo 类型：CLI 工具的 GUI 壳

故事线："给 agent 一个它已经理解的 CLI，它还你一个桌面应用。"后端命令映射为 typed bindings（`binding-generator-impl` 已具备），UI 是薄壳，T_iter 天然最低。
候选底座的共同特征：命令语义agent 完全掌握、输出结构化、生产价值明确（开发者工具类，长期有 GUI 需求但没人愿意手写）。

## 其他候选方向
- 实时数据/事件面板——会立刻暴露 Go→JS 推送方向（订阅式绑定）的缺口，该方向在桥中尚不存在（当前仅 JS→Go 调用 + 响应）
- 生产力工具（笔记/待办，重本地状态与文件桥）

## 暴露缺口也是目的
写真实应用才会遇得到的框架空洞（scheme handler、Go→JS 推送、导航策略）应作为发现记录下来，反哺 M2/M3 的 scope。

## 前置依赖
M2 诊断（事件流 / `--json` CLI / 无头驱动）——验收标准直接建立在 M2 能力之上。
