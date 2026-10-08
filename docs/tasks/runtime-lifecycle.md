---
id: runtime-lifecycle
title: 生产运行时与开发外壳生命周期完善
type: task
status: backlog
priority: high
owner: ai
tags: [runtime, lifecycle, reliability]
related: [release-v1, dev-harness, m2-diagnostics]
blocked_by: []
created_at: 2026-10-08
updated_at: 2026-10-08
---

# 生命周期完善

对应 release-v1 的 R1，补齐运行时退出、连接回收与故障处理；M2 驱动初审发现的既有缺口在此跟踪。

## 已确认缺口

2026-10-08 m2_reviewer：Shell.Close 未关闭已经 accept 但尚未完成 init 的连接，
且缺少关闭状态防止迟到握手重新挂载会话。当前实际 shell 子进程退出由 OS 回收连接，
不阻断本批 macOS 驱动验收；作为可复用 Shell 生命周期仍需修复。

## 待办与验收

- [ ] 跟踪所有已接受连接（含初始化中连接），Close 关闭并回收。
- [ ] 用受同步保护的关闭状态禁止迟到 init/会话重新挂载，明确重复 Close 行为。
- [ ] 增加 Close 与 init/accept 并发、重复关闭、超时握手测试，race 验证无泄漏/死锁/迟到会话。
- [ ] 后续按 R1 扩展 WebView 对象回收、原生窗口和生产故障策略；不得仅凭 shell 进程退出推断全部生命周期已完成。

尚未开始实现，无新增通过结论。
