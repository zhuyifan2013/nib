---
id: cgo-maintenance
title: CGO 平台层维护成本
type: risk
status: open
probability: medium
impact: medium
mitigation: 平台层保持极薄（每平台 ~1.5k 行），无平台逻辑上浮到 Go 层
tags: [cgo, webview]
related: [system-webview]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# CGO 平台层维护成本

webview 包是唯一的平台相关层，三个 OS 各需独立构建验证（macOS 已通）。
内存管理（C/Go 边界）、构建工具链（Xcode/VSCode Build Tools/GTK）都会持续消耗维护精力。
