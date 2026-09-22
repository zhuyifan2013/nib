<div align="center">

# Nib

**The lightweight cross-platform desktop framework built for the AI era.**

A system-WebView shell with a Go core — 10&nbsp;MB apps, 1.4&nbsp;s compile, and a toolchain
designed to be driven by AI agents, not just humans.

`Go` · `macOS` · `Windows` · `Linux` · MIT

</div>

---

## Table of contents

- [Why Nib exists](#why-nib-exists)
- [Status](#status)
- [Quickstart](#quickstart)
- [Usage](#usage)
- [How it works](#how-it-works)
- [Design principles](#design-principles)
- [Comparison](#comparison)
- [Roadmap](#roadmap)
- [Non-goals](#non-goals)
- [FAQ](#faq)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Why Nib exists

Electron proved web technologies win desktop development — and that shipping a
whole browser with every app costs 100–300&nbsp;MB. Tauri and Wails proved the
system-WebView alternative works. What neither solved is the **AI-era workflow**:

> Code is written by AI. Debugging is done by AI. The bottleneck is no longer
> typing speed — it is **iteration latency**: generate → compile → observe → fix,
> hundreds of times a day.

Nib is designed around one north-star metric: **T_iter (time per AI iteration) < 5&nbsp;s**.

Every design decision follows from it:

| Variable | What it means | Nib's answer |
|---|---|---|
| `t_compile` | incremental compile time | Go backend; **<1.5&nbsp;s** (hard requirement, enforced by the CLI) |
| `t_launch` | app startup between iterations | persistent dev shell: window never dies, backend hot-swaps (**≈0**) |
| `t_observe` | seeing what the app did | runtime event stream, headless driver, one-command screenshots |
| `t_context` | understanding failures | cross-boundary **error envelope** — one structured error with the full causal chain, from the JS call stack through IPC to the Go frame |

## Status

> **M1 — minimal runtime.** The macOS chain is verified end-to-end.
> Binding generator, dev harness, and Windows/Linux support are next.

**Working today:** windows, JS bridge, IPC protocol, error envelope, Go bindings,
base64 response transport. Verified by [`cmd/demo`](cmd/demo/main.go).

**Planned:** see [Roadmap](#roadmap).

## Quickstart

Requires **Go 1.23+** and, on macOS, Xcode Command Line Tools.

```sh
git clone https://github.com/zhuyifan2013/nib.git
cd nib
go run ./cmd/demo
```

A window opens. Click the button to see the full round-trip:
`JS bridge → IPC envelope → Go binding → structured response → Promise`.

## Usage

Write your backend in Go, your UI in any web framework (or plain HTML):

```go
package main

import (
	"encoding/json"

	"nib.dev/nib/core"
	"nib.dev/nib/ipc"
)

func main() {
	rt := core.New(core.Options{Title: "My App", Width: 1024, Height: 768})

	rt.Bind("GreetService.Greet", func(args json.RawMessage) (any, *ipc.Error) {
		var a struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, ipc.NewError("greet/invalid_args", err.Error())
		}
		return "Hello, " + a.Name + "!", nil
	})

	if err := rt.Run("<h1>Hello from Nib</h1>"); err != nil { // URL or inline HTML
		log.Fatal(err)
	}
}
```

```js
const result = await window.nib.invoke("GreetService.Greet", { name: "AI" });
// → "Hello, AI!"
```

Errors are structured across the boundary — never strings:

```js
try {
  await window.nib.invoke("GreetService.Greet", { name: 42 });
} catch (e) {
  e.code;      // "greet/invalid_args"
  e.chain;     // [{layer:"js",...},{layer:"ipc",...},{layer:"go",...}]
  e.retryable; // false
  e.doc;       // "https://nib.dev/errors#greet-invalid-args"
}
```

## How it works

```
┌────────────────────────────────────────────────────────┐
│ Tooling      CLI (--json everywhere) · headless driver  │  planned (M2)
│              · MCP server                               │
│ Security     permission annotations · dependency audit  │  planned (M3)
├────────────────────────────────────────────────────────┤
│ API surface  binding generator (Go AST → TS) · error    │  generator planned
│              envelope · call / stream / event IPC       │
├────────────────────────────────────────────────────────┤
│ Dev harness  persistent shell · backend hot-swap · HMR  │  planned (M1)
├────────────────────────────────────────────────────────┤
│ Runtime      Go core + system WebView bindings          │  ✅ macOS today
│              (WKWebView / WebView2 / WebKitGTK)         │
└────────────────────────────────────────────────────────┘
```

| Package | Layer | Responsibility |
|---|---|---|
| [`ipc/`](ipc/protocol.go) | **Protocol** | Single source of truth for JS ↔ Go: the error envelope (`code`, `message`, `chain[]`, `retryable`, `doc`) and the message envelope (`call` / `stream` / `event`). |
| [`core/`](core/runtime.go) | **Abstraction** | Platform-independent `Runtime` / `Window`. Bindings are registered before `Run()` and applied at window creation. |
| [`webview/`](webview/webview_darwin.go) | **Platform bindings** | The *only* platform-specific code — thin CGO wrappers over the system WebView, plus the injected JS bridge (`window.nib.invoke`). ~1.5k lines per platform, nothing platform-specific leaks upward. |
| `binding/` | **Generator** *(planned)* | Parses Go services with `go/ast` and emits a type-safe TypeScript client. |
| `cli/` | **Toolchain** *(planned)* | `nib new / dev / build / doctor / ship / audit` — every command speaks `--json` so AI agents can drive it. |
| [`cmd/demo`](cmd/demo/main.go) | **Demo** | Minimal end-to-end app; the acceptance test for every platform port. |

**IPC transport.** Frontend calls are JSON messages posted via
`window.webkit.messageHandlers`. Responses are the marshalled envelope,
base64-wrapped, delivered through `evaluateJavaScript` — no string-escaping
minefield, no eval-injection surface.

**Why base64?** Inline JSON inside a JS string literal needs multi-layer escaping
(quotes, backslashes, control characters, Unicode). Base64 has no escape surface.
The ~33% size overhead is irrelevant for IPC-sized payloads.
([ADR](docs/decisions/bridge-base64-response.md))

## Design principles

1. **T_iter is the product.** Features that don't move compile, launch, observe,
   or context time wait in the backlog.
2. **Small, predictable API surface.** Strict naming conventions (`fs.read`,
   `win.create`), convention over configuration. An LLM should be able to *guess*
   our APIs — low hallucination surface matters when code is AI-written.
3. **Machine-readable everything.** Structured CLI output, structured errors,
   structured event streams. Half of our users are agents.
4. **Permissions by declaration, default-deny** *(M3)*. AI-written code can't all
   be reviewed — so every native call goes through an explicit, auditable channel.
5. **The platform layer stays thin.** Three WebViews, ~1.5k lines each. Text
   layout, IME, and accessibility are delegated to the OS — problems we refuse
   to own.

## Comparison

| | Electron | Tauri | Wails | **Nib** |
|---|---|---|---|---|
| Bundle size | 100–300 MB | 3–12 MB | 8–15 MB | **<10 MB** (demo: 3.8 MB) |
| Idle memory | 200–500 MB | 40–80 MB | 50–90 MB | **<50 MB** |
| Cold start | 2–5 s | 0.3–1 s | 0.5–1.5 s | **<1 s** |
| Incremental compile | fast | slow (340 s first build) | ~12 s | **1.4 s** |
| Backend | Node.js | Rust | Go | **Go** |
| Type-safe bindings | manual | optional | automatic (call only) | **automatic (call/stream/event)** |
| Error context across IPC | strings | strings | strings | **structured causal chain** |
| Agent-drivable toolchain | — | — | — | **by design (`--json`, headless, MCP)** |
| Mobile | — | ✅ | experimental | planned |

## Roadmap

- [x] **M1 core runtime** — macOS WKWebView binding, IPC protocol, error envelope, demo verified end-to-end
- [x] IPC error-path tests ([task](docs/tasks/ipc-error-path-test.md))
- [x] Binding generator — Go AST → TypeScript, `call` / `stream` / `event` ([task](docs/tasks/binding-generator.md))
- [ ] Dev harness — persistent shell + backend hot-swap (<2 s iteration)
- [ ] Windows (WebView2) and Linux (WebKitGTK) bindings
- [ ] **M2 diagnostics** — runtime event stream, `--json` CLI, headless driver, MCP server
- [ ] **M3 security** — permission annotations (default-deny), dependency audit, `dev --sandbox`
- [ ] **M4 distribution** — signed delta updates with rollback, declarative UI hot-delivery

## Non-goals

- **No built-in AI runtime.** No llm abstraction, no model management. Nib makes
  the *channels* AI needs excellent (streaming IPC, codegen, agent tooling) — it
  does not bundle AI itself. ([ADR](docs/decisions/no-ai-runtime.md))
- **No Chromium forking.** We borrow the OS WebView and accept its fragmentation
  instead of owning a browser.
- **No mobile in v1.** Desktop first; the architecture leaves room.

## FAQ

**Why Go and not Rust (like Tauri)?**
Compile speed is the first variable of T_iter. Rust's 340-second first build is a
tax on every AI iteration; Go's is ~1.4 s. We accept the trade-offs (GC pauses in
the milliseconds, a few extra MB) consciously. ([ADR](docs/decisions/go-backend.md))

**How is Nib different from Wails?**
Same architectural family — Nib is what Wails looks like when designed for
agent-driven development: structured errors with causal chains instead of
strings, three binding kinds including streams, a permission model, built-in
delta updates, and a toolchain that agents can operate. See
[Wails pain points → our answers](docs/tasks/minimal-runtime-mac.md).

**Can I use React / Vue / Svelte?**
Yes. Nib is frontend-agnostic — anything that runs in a WebView runs in Nib.
The binding generator will emit TypeScript clients for any of them.

**Is it production-ready?**
No — M1. The macOS runtime chain is verified, but expect breaking changes until
v1. Track progress in [`docs/tasks/`](docs/tasks/).

**What about WebView differences across platforms?**
Real and accepted. Our mitigation is detection, not uniformity: `nib doctor`
checks the environment, the runtime probes WebView capabilities at startup, and
below-threshold versions get a clear upgrade prompt. ([risk](docs/risks/webview-fragmentation.md))

## Documentation

The project runs on [Markdash](https://github.com/zhuyifan2013/nib/tree/main/docs) —
the dashboard is the docs:

- [`docs/index.md`](docs/index.md) — start here
- [`docs/notes/design-v0.1.md`](docs/notes/design-v0.1.md) — full design document
- [`docs/decisions/`](docs/decisions/) — architecture decision records
- [`docs/tasks/`](docs/tasks/) — current board
- [`docs/risks/`](docs/risks/) — known hazards

Run `markdash serve` for the live dashboard.

## Contributing

Early stage, built in the open. Before writing code:

1. Read [`docs/index.md`](docs/index.md), then only the task/decision docs
   relevant to your change.
2. Respect the decisions in [`docs/decisions/`](docs/decisions/) — they are
   constraints, not suggestions.
3. After doc changes, run `markdash validate`.

## License

[MIT](LICENSE)
