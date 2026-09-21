---
id: webview-fragmentation
title: 系统 WebView 跨平台碎片化
type: risk
status: open
probability: medium
impact: medium
mitigation: doctor 环境检查 + 运行时能力探测 + 启动时版本阈值提示
tags: [webview, cross-platform]
related: [system-webview]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# WebView 碎片化

Windows/macOS/Linux 三个 WebView 内核版本与能力不一（Linux WebKitGTK 最碎）。
同一前端代码可能需要兼容性测试与降级处理。这是借系统 WebView 的原生代价，
缓解目标是"可检测、可提示"，不追求完全一致。
