---
id: dev-workflow
title: 开发环境与本机工作流
type: note
status: active
tags: [operations, macos]
related: []
created_at: 2026-09-21
updated_at: 2026-09-22
---

# 开发环境与本机工作流

## 环境
- macOS arm64，Go 1.27（Homebrew），Xcode CLT
- 必需框架：Cocoa / WebKit（cgo LDFLAGS 已配置）

## 常用命令
- 开发循环（推荐）：`go run ./cmd/nib dev ./cmd/demo` — 窗口常驻，改业务代码 ≈1.1s 热替换
- 生成 TS 绑定：`go run ./cmd/nibgen -out <dir> <pkgdir>`
- 构建 demo：`go build -o bin/demo ./cmd/demo`（bin/ 已 gitignore）
- 运行 demo（一次性）：`go run ./cmd/demo`
- 调试 shell/桥接：`go run ./cmd/nib probe --sock <path>`（模拟业务进程）
- 文档校验：`markdash validate`

## 已知注意事项
- `go run ./cmd/demo` 这类一次性 GUI 进程不要挂在临时 shell 会话里（会被连带清理）；
  开发期用 `nib dev`，其 shell 窗口由编排器托管
- 改 webview/webview_darwin.m 后需重新构建；仅 Go 文件改动走增量编译（~1.4s）
- `nib dev` 状态目录 `.nib/`（socket、构建产物）已 gitignore
- unix socket 路径受 macOS sun_path 104 字节限制，`.nib/shell.sock` 形态安全
