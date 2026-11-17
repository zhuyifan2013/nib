---
id: cli-json
title: CLI 结构化输出契约
type: doc
status: active
owner: ai
tags: [cli, diagnostics]
related: [cli-structured-output, m2-diagnostics]
created_at: 2026-11-17
updated_at: 2026-11-17
---

# CLI 结构化输出

`nib dev`、`shell`、`probe`、`doctor` 和 `nibgen` 支持 `--json`。短命令输出一条 JSON，长命令按行输出 NDJSON。每条记录的 `schema` 为 `nib.cli/v1`，`command` 为命令名，`kind` 为 `lifecycle`、`result` 或 `error`，`status` 表示状态。可选 `data` 随命令和状态变化，失败的 `error` 包含 `code`、`message` 和 `hint`。结束记录包含整数 `exit_code`。

长命令可发出 `starting`、`building`、`started`、`running`、`rebuilding`、`rebuild_failed`、`restarted` 和 `stopped` 生命周期记录，最终以 `result/ok` 或 `error/failed` 结束。dev 的 `started` 表示进程已启动，驱动就绪需查询驱动 API。客户端应逐行解析并检查末条记录和进程退出码。诊断与子进程日志写 stderr，`--json` 与 `--events -` 互斥，事件可写入指定文件。`--` 终止选项解析。

退出码 0 表示成功，1 表示执行失败或依赖缺失，2 表示参数错误，3 表示平台运行时尚未实现。错误码包括 `cli/unknown_command`、`cli/invalid_flag`、`cli/invalid_args`、`cli/output_conflict`、`dev/build`、`dev/runtime`、`shell/runtime`、`probe/connect`、`probe/runtime`、`nibgen/load`、`nibgen/no_services`、`nibgen/generate`、`doctor/dependencies` 和 `doctor/not_implemented`。

`nib doctor --json` 报告 Go、目标架构、CGO、编译器及平台依赖检查结果。`available` 表示对应检查通过，`missing` 表示检查失败，`unknown` 表示未验证。macOS 的 GUI session 和实际窗口启动需要单独验证，Windows/Linux 的运行时仍未实现。诊断驱动 token 通过 `NIB_DRIVER_TOKEN` 提供，输出会遮盖该值。
