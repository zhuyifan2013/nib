# Nib

> AI 时代的轻量跨平台桌面框架：系统 WebView 壳 + Go 核心。

- 安装包 <10MB（借系统 WebView，不打包浏览器）
- 增量编译 <1.5s，开发热替换 <2s（北极星指标：AI 单次迭代时间 T_iter <5s）
- API 面为 LLM 优化，工具链为 AI agent 设计（结构化 CLI / 无头驱动 / MCP）
- 面向不可信代码的安全模型（权限声明 / 依赖审计 / 沙箱报告）

设计文档见 [doc/nib-framework-design-v0.1.md](doc/nib-framework-design-v0.1.md)。

## 仓库结构

| 目录 | 说明 |
|------|------|
| `doc/` | 设计文档 |
| `ipc/` | JS ↔ Go 通信协议（错误信封、消息信封） |
| `core/` | 平台无关的运行时抽象（Runtime / Window） |
| `webview/` | 系统 WebView 的平台绑定（darwin / windows / linux） |
| `binding/` | 绑定生成器（Go AST → TypeScript，call/stream/event） |
| `cli/` | `nib` CLI（new/dev/build/doctor/ship/audit） |
| `cmd/` | 命令入口 |

## 状态

M1 进行中：最小运行时（macOS WKWebView 绑定 + IPC 协议骨架）。
