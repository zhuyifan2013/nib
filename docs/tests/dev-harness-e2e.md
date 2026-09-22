---
id: dev-harness-e2e
title: Dev Harness 端到端热替换验证
type: test
status: passing
kind: e2e
tags: [m1, dev-experience]
related: [dev-harness, dev-harness-impl]
created_at: 2026-09-22
updated_at: 2026-09-22
---

# Dev Harness 端到端热替换验证

覆盖：`nib dev` 常驻外壳的启动、IPC 转发、热替换、构建失败降级。
对应任务 `dev-harness`，方案见 `dev-harness-impl`。

## 环境
macOS（Apple Silicon），Go 1.27，仓库根目录。

## 步骤与结果（2026-09-22）

1. **启动**：`nib dev ./cmd/demo` → shell 窗口拉起，业务进程连接
   （日志：绑定 3 个方法），demo 页面加载并自动跑错误路径自检，`pass=true`。
2. **热替换**：21:35:45.95 改 Boom 错误消息 → 21:35:47 新进程完成自检并输出新消息，
   **端到端 ≈1.1s（<2s 底线）**，shell 窗口全程未退出、无闪烁。
3. **构建失败降级**：追加语法错误 → 日志输出编译错误"构建失败，保留旧进程"，
   旧进程继续服务；修复后自动热替换恢复。
4. **退出**：Ctrl-C 编排器 → 业务进程与 shell 依次退出，socket 清理。

## 单测覆盖
`go test ./harness`：shell 转发（fake 窗口 + 真实 socket）、离线可重试错误、
控制消息、Remote 端到端、watcher 变更检测/去抖/隐藏目录跳过。

## 复现
```sh
go run ./cmd/nib dev ./cmd/demo   # 改 cmd/demo/main.go 观察热替换
go test ./harness/                # 无 GUI 单测
```
注：验证需 macOS GUI 会话；shell 窗口在 dev 会话前台运行时最稳定。
