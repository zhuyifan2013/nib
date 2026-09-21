# Nib

**The lightweight cross-platform desktop framework for the AI era.**
系统 WebView 壳 + Go 核心 — 面向 AI 工作流设计的桌面应用框架。

Nib renders your UI with the operating system's built-in WebView and runs a Go
backend — no bundled Chromium, no bundled Node.js. It is designed around one
north-star metric: **how fast an AI agent can iterate** (generate → compile →
observe → fix).

## Why

Electron proved that web tech wins desktop development — and that shipping a
whole browser with every app costs 100–300MB. Tauri and Wails proved the
system-WebView alternative works. What neither solved is the *AI-era* workflow:
agents write the code, agents do the debugging, and every second of compile
time or missing observability multiplies across hundreds of iterations per day.

| | Electron | Tauri | Wails | **Nib (target)** |
|---|---|---|---|---|
| Bundle size | 100–300 MB | 3–12 MB | 8–15 MB | **<10 MB** |
| Idle memory | 200–500 MB | 40–80 MB | 50–90 MB | **<50 MB** |
| Cold start | 2–5 s | 0.3–1 s | 0.5–1.5 s | **<1 s** |
| Incremental compile | fast | slow (340 s first build) | ~12 s | **<1.5 s** |

Measured on the M1 demo: **1.4 s compile, 3.8 MB binary** (macOS arm64).

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ Tooling      CLI (--json) · headless driver · MCP    │  (planned, M2)
│ Security     permission annotations · dep audit      │  (planned, M3)
├─────────────────────────────────────────────────────┤
│ API surface  binding generator · error envelope      │  (generator: planned)
├─────────────────────────────────────────────────────┤
│ Dev harness  persistent shell · backend hot-swap     │  (planned)
├─────────────────────────────────────────────────────┤
│ Runtime      Go core + system WebView bindings       │  ← works today (macOS)
└─────────────────────────────────────────────────────┘
```

Every layer is a separate package with a narrow job:

| Package | Layer | What it does |
|---|---|---|
| `ipc/` | **Protocol** | The single source of truth for JS ↔ Go communication: the cross-boundary error envelope (code, message, causal chain, retryable, doc link) and the message envelope covering `call` / `stream` / `event`. |
| `core/` | **Abstraction** | Platform-independent `Runtime` and `Window` interfaces. Bindings are registered before `Run()` and applied when the window is created. Platform files (`runtime_darwin.go`, …) wire in the concrete webview. |
| `webview/` | **Platform bindings** | The *only* platform-specific layer — thin CGO wrappers around the system WebView: WKWebView (macOS), WebView2 (Windows, planned), WebKitGTK (Linux, planned). Includes the injected JS bridge (`window.nib.invoke`). |
| `binding/` | **Generator** *(planned)* | Go AST → TypeScript client generation, so frontend calls are fully type-safe. Three first-class binding kinds: `call`, `stream` (async generators with back-pressure), `event`. |
| `cli/` | **Toolchain** *(planned)* | The `nib` command: `new` / `dev` / `build` / `doctor` / `ship` / `audit`. All output is machine-readable (`--json`) so AI agents can drive it. |
| `cmd/demo` | **Demo** | Minimal end-to-end app used to verify the full chain on every platform. |

### The error envelope

One error format across the JS ↔ Go boundary, with the causal chain stitched:

```json
{
  "error": {
    "code": "fs.read/permission_denied",
    "message": "read /etc/passwd: operation not permitted",
    "chain": [
      {"layer": "js",  "frame": "App.tsx:42 handleClick"},
      {"layer": "ipc", "call": "fs.read"},
      {"layer": "go",  "frame": "fs_service.go:88 os.Open"}
    ],
    "retryable": false,
    "doc": "https://nib.dev/errors#fs-permission"
  }
}
```

## Quickstart

Requires Go 1.23+ and (on macOS) Xcode Command Line Tools.

```sh
go run ./cmd/demo
```

A window opens; click the button to see the full round-trip:
JS bridge → IPC envelope → Go binding → structured response.

## Roadmap

- **M1 — minimal runtime** (in progress): macOS chain verified ✅ · error-path tests · binding generator · dev harness (persistent shell + backend hot-swap) · Windows/Linux bindings
- **M2 — diagnostics**: runtime event stream, `--json` CLI, headless driver (`--headless --driver-port`), MCP server
- **M3 — security**: declarative permission annotations (default-deny), dependency audit, `dev --sandbox` syscall reports
- **M4 — distribution**: built-in delta updates (signed, atomic, rollback), declarative UI hot-delivery

Explicit non-goals: no built-in AI runtime (llm abstraction, model management) —
the framework makes *channels* for AI good (streaming IPC, codegen), it does not
bundle AI itself. No Chromium forking. No mobile in v1.

## Documentation

The project dashboard lives in [`docs/`](docs/index.md) (Markdash): design doc,
task board, decisions (ADRs), risks, and test records. Start at
[`docs/index.md`](docs/index.md).

## Contributing

Early stage — everything is being built in the open. See `docs/tasks/` for the
current board and `docs/decisions/` for the constraints every contribution must
respect. Run `markdash validate` after changing docs.

## License

MIT
