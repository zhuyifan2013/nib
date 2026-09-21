---
id: dev-workflow
title: 开发环境与本机工作流
type: note
status: active
tags: [operations, macos]
related: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 开发环境与本机工作流

## 环境
- macOS arm64，Go 1.27（Homebrew），Xcode CLT
- 必需框架：Cocoa / WebKit（cgo LDFLAGS 已配置）

## 常用命令
- 构建 demo：`go build -o bin/demo ./cmd/demo`
- 运行 demo：`go run ./cmd/demo`
- 文档校验：`markdash validate`

## 已知注意事项
- GUI 进程不要挂在临时 shell 会话里（会被连带清理）；开发期保持持久会话，或使用 `nib dev`（未实现）
- 改 webview/webview_darwin.m 后需重新构建；仅 Go 文件改动走增量编译（~1.4s）
