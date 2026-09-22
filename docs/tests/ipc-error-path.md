---
id: ipc-error-path
title: IPC 错误路径测试
type: test
status: passing
kind: e2e
tags: [m1, ipc]
related: [ipc-error-path-test, error-envelope]
created_at: 2026-09-21
updated_at: 2026-09-22
---

# IPC 错误路径测试

覆盖：不存在绑定（ipc/not_found）、参数解码失败、handler 返回错误。
断言前端 reject 收到含 code/chain 字段的结构化对象。对应任务 `ipc-error-path-test`。

## 结论（2026-09-22，macOS）

passing。cmd/demo 内置自检：页面加载即顺序执行三条错误路径，断言 reject 值为
`typeof e === 'object' && typeof e.code === 'string' && typeof e.message === 'string'`，
并比对错误码；结果经 `Demo.Report` 绑定回传 Go stdout。实测输出：

```
error-path selftest: {"pass":true,"lines":[
  "PASS  调用不存在绑定  code=ipc/not_found  message=no binding for NoSuch.Method",
  "PASS  参数解码失败  code=greet/invalid_args  message=json: cannot unmarshal string ...",
  "PASS  Go handler 返回错误  code=demo/boom  message=intentional failure for error-path test"]}
```

复现：`go build -o bin/demo ./cmd/demo && ./bin/demo`，读取日志 `error-path selftest` 行。
注：bridge 目前仅填充 error 的 code/message，`chain` 字段的填充依赖 M2 诊断能力，届时补充断言。
