---
id: cli-structured-output
title: M2 结构化 CLI 输出与环境自检
type: task
status: in_progress
priority: high
owner: ai
tags: [m2, cli, diagnostics]
related: [m2-diagnostics, cli-first, release-v1]
blocked_by: []
created_at: 2026-10-08
updated_at: 2026-10-08
---

# M2 下一批：结构化 CLI

2026-10-08 无头驱动 review 与修复复核通过，已分配 m2_implementer 进行本批实现；实现后由 m2_reviewer 独立审查。

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
