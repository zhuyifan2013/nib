---
id: dev-harness-impl
title: Dev Harness 实现方案（M1：两进程 socket 转发）
type: decision
status: accepted
tags: [m1, dev-experience, architecture]
related: [dev-harness, dev-harness-complexity, go-backend]
created_at: 2026-09-22
updated_at: 2026-09-22
---

# Dev Harness 实现方案（M1：两进程 socket 转发）

## 背景
`dev-harness` 任务：常驻窗口层永不退出，业务 Go 代码改动后 <2s 热替换（窗口无闪烁、
状态可选保留）。Go 无动态链接，进程内热替换无现成机制，见风险 `dev-harness-complexity`。

## 决定：两进程 + unix socket 转发，业务二进制双模运行

```
nib dev ./cmd/demo                 编排器：构建 → 拉 shell → 拉 app → 监听改动
  ├─ nib shell --sock .nib/s.sock  常驻层：窗口 + WebView + 转发（永不退出）
  └─ .nib/dev-app (NIB_SOCK=…)     可替换层：同一份业务二进制，远程窗口模式
```

1. **业务源码零改动**：core.Run 检测 `NIB_SOCK` 环境变量即切换——存在则
   `harness.Remote` 实现 core.Window（连接 shell 的 socket），不存在则走平台原生窗口。
   `go run ./cmd/demo` 与 `nib dev ./cmd/demo` 跑的是同一份代码。
2. **复用 ipc.Message 信封**：业务 ↔ shell 走 unix socket 换行分隔 JSON；
   `nib.ctrl.*` 为保留控制调用（init/navigate/set_title/resize/eval/close），
   其余调用由 shell 整体转发给业务进程（webview 新增 `SetForward` 钩子）。
3. **热替换 = 重建 + 换进程**：轮询 .go 签名（250ms + 300ms 去抖）→ 增量构建
   （~1.4s）→ SIGTERM 旧业务进程 → 起新进程重新 init。**窗口全程不死**：
   前端 JS 状态保留；业务进程离线期间前端调用收到可重试的
   `harness/app_unavailable`（retryable=true，AI 可安全重试）。
4. **构建失败不杀旧进程**：编译错误打到终端，旧业务进程继续服务。
5. **AppKit 线程约束**（实现中发现的硬约束）：任何 goroutine 都可能触达窗口操作
   （如 shell 转发 goroutine 回送响应），所有 ObjC 窗口操作经
   `dispatch_async(dispatch_get_main_queue())` 派发；`webview.New` 用
   `runtime.LockOSThread()` 锁定主 goroutine 跑 `[NSApp run]`。

## 替代方案
- go plugin：macOS 支持但脆弱（依赖版本一致、构建慢），放弃。
- 进程内 goroutine 组重启：无法装载新代码，真热替换需 plugin 或解释器，放弃。
- 前端 HMR dev server 注入：M1 不内置，Navigate 本就支持 http(s) URL，前端框架
  的 dev server 可直接用。

## 影响
- 验收底线达成：改一行业务代码 → 界面更新 ~1.1s（编辑 21:35:45.95 → 新进程自检
  21:35:47），窗口无闪烁；实测记录见 `dev-harness`（task）与 `dev-harness-e2e`（test）。
- 业务进程状态不保留（进程级替换）；"状态可选保留"目前指前端窗口/JS 状态。
  进程内状态快照是理想形态，留待后续按需逼近（风险 `dev-harness-complexity` 保持 mitigated）。
- 为 M2 无头驱动铺路：shell 的转发层即未来 headless 驱动的挂载点。
