# CLAUDE.md

Guidance for working in this repository.

## What this is

`cerebro` is a Go TUI (Charmbracelet stack) for chatting with **local** LLMs. It defaults to
Gemma 4 26B-A4B served by a cerebro-managed `mlx_vlm.server` (OpenAI-compatible), behind a
pluggable provider abstraction.

## Commands

```bash
go run ./cmd/cerebro        # launch the TUI
go test ./...               # run all unit tests (no model/network needed)
go build -o bin/cerebro ./cmd/cerebro
make setup                  # one-time: create venv + install mlx-lm/mlx-vlm (needs uv, network)
gofmt -w . && go vet ./...
```

`go test ./...` is the inner loop and is self-contained. **Live inference needs Apple Silicon
(Metal)** and a running backend, so it can't run in a sandbox without GPU access — validate it
by actually running the app.

## Architecture & where things live

- `internal/llm/` — `Provider` interface (`provider.go`), driver-style `registry.go`
  (`Register`/`New`), and `manager.go` (probe/launch/health-poll/shutdown of the child server).
- `internal/llm/openai/`, `internal/llm/ollama/` — concrete providers. Each **self-registers**
  in `init()`; `internal/app` imports them for side effects (`_ "…/openai"`). To add a backend:
  implement `llm.Provider`, call `llm.Register("kind", factory)`, add the blank import.
- `internal/config/` — `config.yaml` load/save/validate; `ActiveSpec()` maps a model entry to
  `llm.ModelSpec`. `Default()` and the commented first-run template live here.
- `internal/prompts/` — header-prompt `Store` (CRUD + active selection) and the **pure**
  `Assemble(header, history, user) []llm.Message`.
- `internal/history/` — JSON session/conversation persistence.
- `internal/paste/` — pure `Buffer.Collapse`/`Expand` for the `[Pasted Content X Lines]` feature.
- `internal/images/` — image path detection + loading to `llm.Image`.
- `internal/paths/` — XDG locations; **dependency-free leaf** (avoids import cycles). Both
  `config` and `llm` import it.
- `internal/tui/` — Bubble Tea: `model.go` (state), `update.go` (events + streaming),
  `view.go` (layout/render), `commands.go` (slash commands + header editor), `styles.go`, `keys.go`.
- `internal/app/` — wiring (`app.go`) and venv setup (`setup.go`). `cmd/cerebro/main.go` is flags.

## Key conventions & gotchas

- **No import cycles:** `llm` must not import its subpackages (they import `llm`). The registry
  uses self-registration to keep that direction one-way; shared paths go through `internal/paths`.
- **Streaming:** providers return `<-chan llm.Chunk`; the TUI drives it with a re-issuing
  `waitChunkCmd` and cancels via `context`. Keep providers non-blocking on send and honor `ctx`.
- **Enter vs newline:** the textarea's `InsertNewline` is rebound to `ctrl+j`; the root model
  handles `enter` as submit. Don't forward `enter` to the textarea.
- **Paste:** detected via `tea.KeyMsg.Paste` (bracketed paste). Display is collapsed; the full
  text is restored with `pasteBuf.Expand` before sending. Transcript shows display text;
  `chatMessage.modelText` carries the expanded text for context.
- **Config/prompt files** are written to XDG paths; tests set `XDG_CONFIG_HOME`/`XDG_DATA_HOME`
  to a temp dir, or pass explicit paths to `Load`/`Save`.
- **Backend lifecycle:** `llm.Manager` launches the server in its own process group and kills it
  on exit (`defer manager.Shutdown()` in `app.Run`). Child output goes to `server.log`.

## Testing notes

Add tests next to the package. Use `httptest.Server` for providers (see
`internal/llm/openai/openai_test.go`). Prefer pure functions for new non-UI logic so they stay
testable without the TUI.
