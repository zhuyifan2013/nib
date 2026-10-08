---
id: driver
title: macOS 本机驱动 API 使用
type: doc
status: active
tags: [m2, driver, operations]
related: [local-driver, driver-e2e, m2-diagnostics]
created_at: 2026-10-08
updated_at: 2026-10-08
---

# 启用与验证

在 macOS 已登录 GUI session 中：

```sh
go build -o /tmp/nib ./cmd/nib
export NIB_DRIVER_TOKEN="$(openssl rand -hex 24)"
/tmp/nib dev --nowatch --headless --driver-port 8123 ./cmd/demo
```

另一个终端用同一个 token：

```sh
curl -H "Authorization: Bearer $NIB_DRIVER_TOKEN" http://127.0.0.1:8123/v1/ready
curl -H "Authorization: Bearer $NIB_DRIVER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"call":"GreetService.Greet","args":{"name":"AI"}}' http://127.0.0.1:8123/v1/call
```

直接应用设置 core.Options{Headless:true, Driver:driver.Options{Port:8123, Token:token}}。harness 模式用 CLI flags，应用内 Driver.Port 保持 0。端口设为 0 时完全关闭驱动。--headless 与端口开关独立。

`nib dev --driver-timeout 1500ms` 可设置请求超时，原样传给内部 `nib shell --driver-timeout`；core/DevOptions 同样可设 Driver.Timeout。0 表示默认 5s，合法范围为正值至 30s；负值或超过 30s 在构建业务程序/创建 shell socket 前被拒绝。token 始终只经 Options/env 传递。

| 方法/路径 | JSON 请求 | 返回 |
|---|---|---|
| GET /v1/ready | 无 | result.ready/url（页面 complete 且桥可用） |
| GET /v1/events?after=0 | 无 | result.events/oldest_seq（最多环形缓冲容量，默认 256） |
| POST /v1/navigate | {"target":"URL 或 <html>"} | result.accepted；随后检查目标页面 |
| POST /v1/eval | {"script":"Promise.resolve(42)"} | result 为 JSON 值 |
| POST /v1/call | {"call":"Service.Method","args":{}} | result；Go 错误是 driver/operation，message 保留错误信封 JSON |
| POST /v1/dom | {} | result.html/title/url/readyState |
| POST /v1/click | {"selector":"#btn"} | result:true；仅 DOM click |
| POST /v1/fill | {"selector":"#input","value":"text"} | result:true；input/textarea/select + input/change 事件 |
| POST /v1/screenshot | {} | image/png 二进制 |

通用错误形态：{"error":{"code":"driver/…","message":"…"}}。未鉴权 401、Origin 403、非法参数 400、错误方法 405、未知路径 404、并发忙 429、JS/原生失败 422、超时 504。readiness 只表示当前文档就绪，不代表业务首次连接完成；启动时需检查 app_connected 或可调用绑定。

可复现端到端验证：`python3 scripts/driver_e2e.py`。脚本构建本地 fixture，验证直接运行和 dev shell 两条路径，在临时目录留下日志和 PNG；不访问外网。单元验证：`go test ./...`、`go test -race ./driver ./events ./harness`。

driver token 不要传到页面、URL 或公开日志。读取事件可包含业务错误/栈数据，诊断仅用于可信本机开发。隐藏窗口不是无显示器运行支持；其余限制见 local-driver。
