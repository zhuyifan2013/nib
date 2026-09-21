---
id: system-webview
title: 渲染用系统 WebView，不打包浏览器
type: decision
status: accepted
tags: [architecture, rendering]
related: [go-backend, webview-fragmentation]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 渲染用系统 WebView

## 决定
借用系统组件：Windows → WebView2，macOS → WKWebView，Linux → WebKitGTK。

## 理由
- 体积从 Electron 的 100-300MB 降到 <10MB 的唯一现实路径
- 文本排版/IME/无障碍免费获得（自研渲染会死在输入法上）
- Electron（重但一致）与 Compose/Skia（一致但重）两条路都被排除

## 代价与对策
跨平台渲染差异 → 风险 `webview-fragmentation`：doctor 检查 + 运行时能力探测，接受"可检测可提示"而非"完全一致"。
