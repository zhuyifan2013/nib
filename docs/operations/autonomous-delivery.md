---
id: autonomous-delivery
title: 自主交付编排与交接
type: doc
status: active
owner: ai
tags: [workflow, release, agents]
related: [release-v1, m2-diagnostics]
created_at: 2026-10-08
updated_at: 2026-11-17
---

# 自主交付编排

2026-10-08 用户授权主 agent 统筹 Nib 正式发布路线图，安排一个实现 subagent，
完成后安排另一个独立 agent review，修复并验收后直接进入下一批，不需要日常确认。

## 每批流程

1. 主 agent 从 release-v1 选取最小可交付批次，明确依赖、范围、接口与验收证据。
2. 单个实现 agent 修改代码并完成相关验证，更新任务/测试/决策文档。
3. 实现结束后安排独立 reviewer；reviewer 默认只读，检查实际 diff、需求、失败路径、安全、兼容性与测试证据，不采信实现 agent 的结论作为通过依据。
4. 有问题则交给实现 agent 修复，由 reviewer 复核；未解决的发布阻断项不得通过。
5. 主 agent 复核必要检查与证据，更新完成情况、限制和下一批；继续路线图。

共享 checkout 同一时间仅一个实现 agent 修改相同代码；review 时暂停该批编辑。
保留既有改动，禁止 reset/覆盖用户工作。主 agent 负责跨任务文档与整体状态。

## 自动推进与阻塞处理

当前聊天的自动接续任务 `nib`（Nib 自主交付与独立审查）已按用户要求 PAUSED。
2026-10-08 用户要求当前 CLI --json 与 doctor 批次完成后暂停，原因是 token 消耗过快。
CLI 批次已于 2026-11-17 完成独立 review 和验收，按原交接在本批后暂停。
自动接续已先暂停，避免额外触发；恢复必须由用户明确要求。
保留每 10 分钟的调度配置。检查现有执行状态后推进，避免重复启动；仅报告批次完成、
重要问题、所需外部条件或整体完成。本地任务要求电脑开机、应用运行、项目目录可用。

- 常规设计、编码、测试、修复和文档更新自行完成。用户新消息可以纠正优先级或范围。
- 独立 review 不代替测试；本机测试不代替 Windows/Linux 实机/native CI；隐藏窗口不代替无显示器验证。
- 无平台环境、账号或签名凭据时，如实记录待验收项，推进其他无依赖工作，不假报完成。
- 不因写出实现就勾选通过；release-v1 的发布验收需要安装、升级和外部干净环境证据。
- 会话/执行环境/额度限制会影响持续运行；不宣称离线或额度耗尽时仍能执行。后续恢复从本文件与任务文档接续。

## 批次台账

| 批次 | 状态 | 实现 | 独立 review | 证据/下一步 |
|---|---|---|---|---|
| M2 增量 2：macOS 无头驱动 | done | m2_implementer | m2_reviewer（初审与修复复核通过） | 单测/race、独立 direct+harness GUI E2E 通过；Timeout P2 修复并实测 1.50s，已验收 macOS 范围 |
| M2 增量 3：结构化 CLI 与 doctor | done | 接续草稿并修复 | 独立 review 与修复复核通过 | 单测、race、TS 类型检查、真实 macOS JSON GUI 闭环通过；详情见 `cli-structured-output` |

独立 reviewer 的 GUI 证据目录：`/var/folders/t0/lg_bhrmd66n7hk05s0_mt5vm0000gn/T/nib-driver-e2e-2j44wo34`。
非阻断的 shell 迟到握手/连接关闭问题已列入 `runtime-lifecycle`，不作为已解决问题。

Timeout 修复复核证据：`/var/folders/t0/lg_bhrmd66n7hk05s0_mt5vm0000gn/T/nib-driver-review-timeout-cy5fej09`；独立非缓存针对单测与 harness GUI E2E 通过。

当前批次：M2 CLI --json 与 doctor 已完成，按交接暂停。
恢复后的下一候选为 MCP，随后资源加载与最小脚手架/构建闭环；目前不启动。
