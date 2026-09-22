---
id: binding-generator-impl
title: 绑定生成器实现路线（M1）
type: decision
status: accepted
tags: [m1, codegen, dx]
related: [binding-generator, dev-harness, error-envelope]
created_at: 2026-09-22
updated_at: 2026-09-22
---

# 绑定生成器实现路线（M1）

## 背景
`binding-generator` 任务落地：用户写 Go 服务方法，CLI 生成类型安全 TS 客户端。
设计稿（docs/notes/design-v0.1.md §5.2）定了三类绑定（call/stream/event）与
"go/ast + go/types → TS 接口 + IPC 存根"的方向，实现细节需拍板。

## 决定
1. **零新增依赖**：go/parser + go/types + importer.Default()，不引入
   golang.org/x/tools。保住"零依赖、编译快"的仓库特性（T_iter 第一变量）。
   代价：跨包类型解析只覆盖已编译依赖；外部包命名类型报错引导用户用本包类型
   （time.Time、json.RawMessage 作为特例映射）。
2. **绑定发现模型**：导出 struct 的导出方法即绑定，方法名 = `Service.Method`；
   `// @binding stream|event` 注解选类型，缺省 call。非导出/非法签名收集后
   一次性报错（不静默跳过）。
3. **类型映射保名**：本包命名结构体 → 生成具名 interface；具名基础类型/切片/map
   （如 `type Status string`）→ 生成 `export type Status = string`，
   不用底层类型替换（保住类型身份与文档跳转）。嵌入结构体展平，omitempty → `?`。
4. **增量生成 = 每服务一文件 + 内容比对**：services/<Svc>.ts 各自独立，
   写盘前比对内容，未变不刷 mtime；已删除服务的残留文件自动清理。
   （设计稿"改一个方法不重刷全部"按文件粒度满足。）
5. **stream/event 客户端先行，运行为桩**：生成完整类型化签名
   （AsyncGenerator / subscribe），shim 在运行时抛 `ipc/not_implemented`；
   协议与生成物先行，运行时随 M4 落地——避免 M1 阻塞在流式 IPC 上。
6. **错误类型内嵌生成物**：nib.ts 自带 NibError/NibErrorEnvelope（对应
   `error-envelope` 决定），call() 把桥 reject 的裸信封规范化为 NibError，
   不依赖外部包。

## 替代方案
- go/packages（golang.org/x/tools）：解析最稳，但一次引入多个间接依赖，放弃。
- 反射运行时绑定（rt.BindService）：注册零代码，但签名约束隐式、错误延迟到
  运行期；生成 Go 胶水代码则双份产物难同步。M1 保持注册手写（rt.Bind），
  生成器只管 TS；反射助手留待 dev-harness 阶段按需补。

## 影响
- 验证：`go test ./binding`（golden + 增量 + 非法签名拒绝）；
  golden 输出经 `tsc --noEmit --strict --noUnusedLocals` 类型检查。
- M4 实现流式 IPC 时只需替换 nib.ts 的 stream/subscribe 桩，生成物不变。
