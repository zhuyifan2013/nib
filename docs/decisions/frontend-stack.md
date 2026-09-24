---
id: frontend-stack
title: UI 层复用 Web 生态，不自研 UI 框架
type: decision
status: accepted
tags: [architecture, frontend, agent-first]
related: [system-webview, go-backend, binding-generator-impl]
created_at: 2026-09-22
updated_at: 2026-09-22
---

# UI 层复用 Web 生态，不自研 UI 框架

## 决定
UI 用任意 Web 前端栈编写（HTML/CSS/JS 及其编译源），Nib 不自研 UI 组件层或声明式 UI 语言。
推荐组合：**Vite + React 或 Vue + Tailwind**，配绑定生成器产出的 TypeScript 客户端。
产物经 `file://` / `nib://` 加载；开发期前端 HMR 走 Vite dev server（`http://localhost:*` 直接 Navigate，已验证可行），后端热替换走 `NIB_SOCK`，两条热路径互不阻塞。

## 理由
- 与 `system-webview` 决策自洽：壳里跑的就是网页，任何能编译成 Web 产物的前端栈天然可用，生态（组件库、模板、工具链）直接白嫖。
- **agent-first**：Web 栈是 AI 训练数据里最稠密的领域，agent 生成/修复 UI 的成功率最高、迭代最快，直接服务北极星指标 T_iter < 5s。自研 UI 层会引入 agent 不熟悉的私有 API，反向拖慢迭代。
- 类型安全闭环已有：`binding-generator-impl`（Go AST → TypeScript）让前端拿到带类型的 `window.nib.invoke(...)`，对标 Wails 的卖点但协议级、语言无关。
- 桌面端 UI 技术（Qt/Flutter/SwiftUI/egui 等"直接渲染"路线）与北极星指标不匹配，排除——不在 Nib 的赛道上重复造轮子。

## 代价与对策
- 远程页面无导航策略时，`window.nib` 桥对任意网站暴露 → 已在 M3 安全层（任务 `m3-security`）覆盖：scheme 白名单、`decidePolicyForNavigation` 拦截。
- `nib://` 自定义 scheme 的 handler（`WKURLSchemeHandler`）尚为空白，本地资源加载方案待 M2/M3 落地时确定。
- 绑定生成器目前只覆盖 Go → TypeScript；其他后端语言的客户端绑定按需求再加（协议是语言无关的）。

## 验证方式
以生产价值的示范性项目（任务 `demo-apps`）做 dogfooding：用推荐栈写真实应用，检验 T_iter 与桥接开发体验。
