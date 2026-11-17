---
id: m2-diagnostics
title: M2 诊断层（事件流 / CLI --json / 无头驱动）
type: task
status: in_progress
priority: high
owner: ai
tags: [m2, observability]
related: [error-envelope, binding-generator, cli-first, demo-apps]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-11-17
---

# M2 诊断层

运行时事件流（窗口/IPC/文件/panic 结构化事件 → inspection 页面 + CLI 流式导出）、CLI 全命令 --json、无头驱动模式（app --headless --driver-port，迷你 CDP：点击/填表/调绑定/截图/DOM 快照）、MCP server。设计依据见 `design-v0-1` 支柱二。决策约束见 `cli-first`。

## 增量分解（按依赖序）

- [x] **增量 1：事件流基础**（done，见 `event-stream` 测试文档）— `events` 包（Bus：seq/环形缓冲/订阅重放）；webview 发 window.*/ipc.* 事件 + handler panic 兜底（recover → runtime.panic 事件 + 结构化错误响应）；shell 侧事件经 `nib.ctrl.events` 转发 + 重放给业务进程（业务进程持有完整流，为驱动端口铺路）；`nib dev/shell --events <path|->` NDJSON 导出。顺手修复 M1 潜伏缺陷：每条消息新建 json.Decoder 导致缓冲数据丢弃（连接级 Session 化）
- [x] **增量 2：无头驱动**（macOS 隐藏窗口实现，独立 review 与修复复核通过） — `--headless --driver-port`：HTTP 驱动 API（navigate/eval 带返回/调绑定/读事件流/截图/DOM 快照）；依赖桥增强（eval-with-result）与原生截图 API
- [x] **增量 3：CLI --json** — dev/shell/probe、doctor 与 nibgen 结构化输出和环境自检，验收见 `cli-structured-output`。
- [ ] **增量 4：MCP server** — 框架运行时挂 MCP，AI 助手可操作应用调试与验证

## 当前进度

增量 1 完成并验证（单测 + apple.com 端到端 NDJSON 导出，测试文档 `event-stream`）。
增量 2 完成：macOS 隐藏窗口、本机鉴权 HTTP API、Promise eval-with-result、真实桥 binding 调用、事件轮询、WKWebView PNG、DOM、CSS click/fill。core 直接模式和 dev 常驻 shell 均已实测；API/安全/运行边界见 `local-driver` 与 `driver`，验收见 `driver-e2e`。独立 review 与 Timeout 传播修复复核通过；不宣称无显示器 CI 或 Windows/Linux 已验证。

增量 3 CLI --json 与 doctor 已完成 macOS 本机验证、独立 review 与复核，详见 `cli-structured-output`；增量 4 MCP 待启动。
