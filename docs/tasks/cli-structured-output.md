---
id: cli-structured-output
title: M2 结构化 CLI 输出与环境自检
type: task
status: done
priority: high
owner: ai
tags: [m2, cli, diagnostics]
related: [m2-diagnostics, cli-first, release-v1]
blocked_by: []
created_at: 2026-10-08
updated_at: 2026-11-17
---

# M2 下一批：结构化 CLI

## 2026-11-17 续接

修复参数解析器将末尾缺值 flag 的内部终止符当作值的问题，以及日志脱敏在缓冲边界可能泄露 token 的问题。GUI 验证发现 macOS 窗口创建偶发不在主线程，已在 webview 初始化时绑定主线程。

`go test ./...`、`go test -race ./cli ./driver ./harness ./events` 通过；绑定生成器示例输出通过 TypeScript 7.0.2 严格类型检查。`python3 scripts/driver_e2e.py` 的直接模式和 harness 模式通过。另用真实 `nib dev --json --nowatch --headless` 启动 demo，驱动调用返回 `Hello, AI! (from Go)`，SIGTERM 后退出码 0，stdout 每行均为 JSON 且末条结果 exit_code 为 0。`nib doctor --json` 在本机 darwin/arm64 报告 Go、CGO、编译器与 macOS SDK 检查通过，GUI 状态为 unknown。输出契约见 `cli-json`。

独立 review 与修复复核通过。审查发现并修复 JSON 字段级脱敏、转义 token 和人类输出脱敏、顶层帮助、Go 最低版本判断、shell 过早报告 running 等问题。dev 的 started 表示进程已启动，驱动就绪通过 `/v1/ready` 查询。独立 reviewer 执行 `go test -count=1 ./cli ./harness ./driver` 通过；修复后再次执行全量测试、race、驱动 E2E 和真实 `dev --json` GUI 调用及正常退出通过。

2026-10-08 无头驱动 review 与修复复核通过，已分配 m2_implementer 进行本批实现；实现后由 m2_reviewer 独立审查。

## 历史中断交接（2026-10-09）

实现 agent 在上一轮遇到使用额度上限而中断；当前没有存活的实现或 review agent。
已保留未提交的 cli/output.go、cli/doctor.go、单测/命令集成测试草稿及
cmd/nib、cmd/nibgen、harness 的接入改动。存在代码不代表已验证完成。
尚无本批最终测试、JSON GUI 验收、独立 review 或验收通过结论。
自动接续仍 PAUSED；用户要求本批完成后暂停，不启动下一批。
后续接续先检查现有草稿，完成必要验证与独立 review，避免重做已验收的驱动批次。

## 范围

- 当前 nib dev/shell/probe 和 nibgen 的 --json 入口与错误输出；未知命令、非法 flag、缺参、运行错误均提供稳定结构化结果。
- 定义并文档化版本化输出 schema、错误码、修复建议与退出码；长时间运行命令使用明确的 NDJSON 生命周期/结果记录。
- JSON stdout 不得混入 logger、业务/子进程普通日志或原始事件导出；诊断与子进程日志走 stderr，--events - 冲突需显式规则。
- nib doctor 首版：Go/编译工具链及平台 WebView 依赖检查；能区分缺依赖、未实现平台和未知/未验证，不因编译一个 CLI 就声称平台运行时支持。
- 常规人类输出与原有 dev/shell/probe 功能保留；token/凭据不能进入日志和 JSON 报告。
- 未实现 new/build/ship/audit 不作为已支持命令，后续实现时必须复用此输出契约。

## 验收

- 命令级集成测试通过独立进程验证 stdout 可解析、stderr 分流、退出码、错误码、参数位置/非法输入、生成失败与构建失败。
- 至少一次真实 dev/shell 驱动 GUI 闭环验证 --json 未破坏启动和退出；doctor 在当前平台给出实际检查证据。
- Go 单测、必要 race 测试、TS 生成器回归通过；文档记录已验证平台与局限。
- 实现结束必须独立 review 和复核后才能标 done。
