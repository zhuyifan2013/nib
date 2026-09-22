---
id: binding-generator
title: 绑定生成器（Go AST → TypeScript）
type: task
status: done
priority: high
owner: ai
tags: [m1, codegen, dx]
related: [dev-harness, error-envelope]
blocked_by: []
created_at: 2026-09-21
updated_at: 2026-09-22
---

# 绑定生成器

## 目标
用户写 Go 方法（含 `// @binding stream|event` 注解），CLI 生成类型安全的 TS 客户端：`call` / `stream`（AsyncGenerator + 背压）/ `event` 三类。

## 要点
- go/ast + go/types 解析；增量生成（改一个方法不重刷全部）
- 生成物含错误类型定义（对应错误信封）
- 这是 Wails 式 DX 的核心引擎，也是 M2 无头驱动的类型基础

## 结果与验证

**Result:** verified — 生成器落地为 `binding/` 包 + `cmd/nibgen` CLI，实现路线见
`binding-generator-impl`（decision）。

- 三类绑定：`call` 全链路可用；`stream`/`event` 生成完整类型化签名（AsyncGenerator /
  subscribe），运行时为 `ipc/not_implemented` 桩（M4 替换，见 decision #5）
- 类型映射：本包命名结构体 → 具名 interface；具名基础类型 → type alias（保名）；
  嵌入展平；omitempty → 可选字段；time.Time → string
- 增量：每服务一文件 + 写盘前内容比对（未变不刷），残留文件自动清理
- 生成物自带运行时 shim（nib.ts）：NibError 错误信封 + call 封装，对应 `error-envelope`

验证：`go test ./binding`（golden 基准 + 增量不重写 + 非法签名拒绝）通过；
golden 输出经 `tsc --noEmit --strict --noUnusedLocals` 类型检查通过；
CLI 实测 `go run ./cmd/nibgen -out <dir> <pkgdir>` 二次运行全 same。

用法：`nibgen -out ./frontend/src/bindings ./backend`。已知限制：跨包命名类型
（time.Time/json.RawMessage 除外）暂不支持，报错引导。
