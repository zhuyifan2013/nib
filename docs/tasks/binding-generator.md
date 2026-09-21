---
id: binding-generator
title: 绑定生成器（Go AST → TypeScript）
type: task
status: todo
priority: high
owner: ai
tags: [m1, codegen, dx]
related: [dev-harness, error-envelope]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-21
---

# 绑定生成器

## 目标
用户写 Go 方法（含 `// @binding stream|event` 注解），CLI 生成类型安全的 TS 客户端：`call` / `stream`（AsyncGenerator + 背压）/ `event` 三类。

## 要点
- go/ast + go/types 解析；增量生成（改一个方法不重刷全部）
- 生成物含错误类型定义（对应错误信封）
- 这是 Wails 式 DX 的核心引擎，也是 M2 无头驱动的类型基础
