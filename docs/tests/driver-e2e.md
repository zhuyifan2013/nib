---
id: driver-e2e
title: M2 本机驱动与隐藏窗口验证
type: test
status: passing
kind: e2e
tags: [m2, driver, macos]
related: [m2-diagnostics, local-driver, driver, event-stream]
created_at: 2026-10-08
updated_at: 2026-10-08
---

# 验证记录

2026-10-08 macOS 本地 GUI session，隐藏 NSWindow。不是无显示器/远程 CI 证据。

- `go test ./...` 通过，新增鉴权/Origin 拒绝、非法/超量/多段 JSON、控制 binding 拒绝、selector/value escaping、超时、事件窗口和参数校验测试。
- `python3 scripts/driver_e2e.py` 直接 core 与 nib dev shell 两条路径通过：ready、未鉴权拒绝、中文 binding 与错误信封、Promise eval、JS 错误/循环值、导航、中文 fill/click、缺失 selector、DOM、真实原生 PNG、IPC 事件、5s 超时、并发 429、超时后恢复。进程退出后端口不再接受连接；shell 通过原生窗口关闭退出，dev 父进程收到 shell 退出并清理业务进程（退出码 0）。
- 首次 smoke 使用 demo，原 IPC 错误路径自检通过；按钮结果为 “Hello, AI! (from Go)”，中文参数结果未乱码。
- E2E 证据目录：`/var/folders/t0/lg_bhrmd66n7hk05s0_mt5vm0000gn/T/nib-driver-e2e-xc08q5_5`（临时证据，以脚本可复现为准）；harness PNG 已实际打开核对，显示本地测试页面内容。
- events 并发 Emit/Unsubscribe 回归测试通过；修复已有发送 closed channel 竞态。最初 race 运行暴露 harness fakeWindow 无锁测试读写；已补锁/快照与清理，并新增 socket readiness 等待及退出/超时回归，最终 `go test -race ./driver ./events ./harness` 通过。

范围限制：截图仅可确认 PNG/实际页面内容，不宣称跨机器像素确定性；未验证 Windows/Linux、无 GUI macOS、可访问性树、热替换长稳与原生多窗口释放。独立代码 review 和修复复核已通过，结论限于本批 macOS 驱动范围。


## 独立 review 修复

2026-10-08 独立 review 指出 DevOptions.Driver.Timeout 在启动 shell 时丢失。修复 dev/shell --driver-timeout 全链路传播；零值保持 5s 默认，负数及超过 30s 提前拒绝。

新增回归覆盖 0/1500ms/30s 的 shell argv 重解析（包含 port/headless/events 且不含 token），负数和超限值在 Dev 首次构建与 shell socket 创建前被拒绝，driver.Start 同样拒绝负数。针对测试 `go test ./driver ./harness ./cmd/nib` 通过。真实 GUI 脚本现在给 harness 设置 1500ms，并断言未完成 Promise 在 1–3.5s 内返回 504，能识别旧实现落回 5s 的缺陷；直接模式继续验证默认超时。最终两条路径通过，harness 实测 1.50s 返回 504；最新证据目录 `/var/folders/t0/lg_bhrmd66n7hk05s0_mt5vm0000gn/T/nib-driver-e2e-pouma351`。

reviewer 已单独复跑原完整 E2E 与 race 通过（证据目录 nib-driver-e2e-2j44wo34）。非阻断 Shell.Close 未完成 CtrlInit 连接/迟到握手问题由后续 runtime-lifecycle 任务跟进，本次未扩大实现范围。

修复复核通过：reviewer 独立执行 `go test -count=1 ./driver ./harness -run 'Test(StartValidation|TimeoutAndQuoting|DevDriverTimeoutPropagation|InvalidDriverTimeoutRejectedBeforeStartup)'`，并重新构建最新 nib 跑 harness GUI E2E；自定义超时实测 1.50s，无新增阻断问题。证据目录 `/var/folders/t0/lg_bhrmd66n7hk05s0_mt5vm0000gn/T/nib-driver-review-timeout-cy5fej09`。主 agent 最终执行 `go test ./...` 通过（缓存），验收增量 2 的 macOS 范围。
