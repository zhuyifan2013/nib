---
id: ipc-error-path-test
title: IPC 错误路径验证
type: task
status: todo
priority: high
owner: ai
tags: [m1, ipc, testing]
related: [demo-e2e-manual, error-envelope]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# IPC 错误路径验证

## 目标
验证三条错误路径前端均收到结构化 `ipc.Error`（而非字符串或崩溃）：
1. 调用不存在的绑定 → `ipc/not_found`
2. 参数解码失败（如 greet/invalid_args）→ 业务错误码透传
3. Go handler 返回错误 → reject 收到 `env.error` 对象

## 做法
扩展 cmd/demo：加"调用不存在方法"和"传错误参数"两个按钮；人工/脚本断言页面显示错误码字段。结论记入 `ipc-error-path`（test）。
