---
id: ipc-error-path-test
title: IPC 错误路径验证
type: task
status: done
priority: high
owner: ai
tags: [m1, ipc, testing]
related: [demo-e2e-manual, error-envelope, ipc-error-path]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-22
---

# IPC 错误路径验证

## 目标
验证三条错误路径前端均收到结构化 `ipc.Error`（而非字符串或崩溃）：
1. 调用不存在的绑定 → `ipc/not_found`
2. 参数解码失败（如 greet/invalid_args）→ 业务错误码透传
3. Go handler 返回错误 → reject 收到 `env.error` 对象

## 做法
扩展 cmd/demo：加"调用不存在方法"和"传错误参数"两个按钮；人工/脚本断言页面显示错误码字段。结论记入 `ipc-error-path`（test）。

## 结果与验证

**Result:** verified — cmd/demo 增加页面内自检（加载即跑三条错误路径 + GreetService.Boom 失败绑定），
断言 reject 对象为结构化 `{code, message}`；结果经 `Demo.Report` 绑定回传 stdout，脚本化断言通过：

```
PASS  调用不存在绑定   code=ipc/not_found
PASS  参数解码失败     code=greet/invalid_args
PASS  Go handler 返回错误  code=demo/boom
```

复现：`go build -o bin/demo ./cmd/demo && ./bin/demo`，查看日志中 `error-path selftest` 行（pass=true）。
详见 `ipc-error-path`（test）。
