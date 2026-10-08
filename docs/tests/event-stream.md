---
id: event-stream
title: 事件流（M2 增量 1）测试
type: test
status: passing
kind: e2e
tags: [m2, observability]
related: [m2-diagnostics, cli-first]
created_at: 2026-09-24
updated_at: 2026-10-08
---

# 事件流（M2 增量 1）测试

覆盖：`events` 包总线语义、harness 跨进程事件转发（重放/实时/离线缓冲）、
`--events` NDJSON 导出、handler panic 兜底。对应任务 `m2-diagnostics` 增量 1。

## Cases / steps

- 单测（`go test ./events/ ./harness/`）：Seq 递增与顺序、环形覆盖、订阅重放、
  慢消费者丢弃、`TestShellStreamsAndReplaysEvents`（上线前重放 / 实时转发保留原 Seq /
  离线缓冲 / 重连补齐）
- 端到端：`nib shell --sock <p> --events <path>` + `nib probe --navigate https://www.apple.com`，
  断言 NDJSON 事件序列完整

## 结论（2026-09-24，macOS）

passing。实测导出（加载 apple.com 并触发一次桥调用）：

```jsonl
{"seq":1,"kind":"window.created","data":{"devtools":true,"height":768,"title":"Nib Dev","width":1024}}
{"seq":2,"kind":"harness.app_connected","data":{"bindings":1}}
{"seq":3,"kind":"window.navigated","data":{"target":"https://www.apple.com"}}
{"seq":4,"kind":"ipc.call","data":{"args_bytes":80,"call":"Probe.Echo","id":"m1"}}
{"seq":5,"kind":"ipc.response","data":{"id":"m1","result_bytes":78}}
```

过程中修复一个 M1 潜伏缺陷：`Recv` 每条消息新建 `json.Decoder`，多条消息同抵达时
缓冲数据随 Decoder 丢弃（表现：事件推送后转发的请求永久丢失）。修复为连接级
`Session`（Decoder 常驻 + 写锁），shell/remote/serveConn 全部会话化。

## 进度复核（2026-10-08，macOS）

`go test ./...` 通过：binding、events、harness 测试均通过（binding/events 使用缓存）。
本次未重跑 GUI 端到端验证；上述 NDJSON 端到端结论仍来自 2026-09-24 的记录。

## 已知边界（增量 2+ 处理）

- `ipc.error` 事件在 shell 转发路径（DeliverResponse）已覆盖；app 本地 handler
  panic 的事件（runtime.panic）已兜底但暂无自动化断言
- 事件 seq 为各进程独立编号空间，跨进程关联以 time 为准
- 驱动端口（`--headless --driver-port`）是事件流的下一个消费方
