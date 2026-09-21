---
id: ipc-error-path
title: IPC 错误路径测试
type: test
status: planned
kind: e2e
tags: [m1, ipc]
related: [ipc-error-path-test, error-envelope]
created_at: 2026-09-21
updated_at: 2026-09-21
---

# IPC 错误路径测试

覆盖：不存在绑定（ipc/not_found）、参数解码失败、handler 返回错误。
断言前端 reject 收到含 code/chain 字段的结构化对象。对应任务 `ipc-error-path-test`。
