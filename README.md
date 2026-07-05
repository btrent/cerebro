# cerebro

A polished, ChatGPT-like terminal interface for **local** LLMs, built in Go with the
[Charmbracelet](https://charm.sh) stack (Bubble Tea, Bubbles, Lip Gloss, Glamour).

It feels similar to Claude Code's chat interface: a scrollable conversation, a multiline
prompt, streaming responses, reusable "header" prompts, slash commands, image attachments,
and graceful keyboard handling — all talking to a local model through a pluggable provider
abstraction.

```
┌ cerebro  Model: Gemma 4 26B-A4B  •  Header: none  •  ● ready ───────────────┐
│ ▌ You                                                                        │
│   Explain goroutines in two sentences.                                       │
│ ▌ cerebro                                                                    │
│   Goroutines are lightweight, runtime-scheduled threads…                     │
└─────────────────────────────────────────────────────────────────────────────┘
┃ Send a message (Enter to send, Ctrl+J for newline, /help for commands)
enter send • ctrl+j newline • esc stop • ctrl+l clear • ctrl+c quit
```

## Default model & backend

cerebro defaults to **Gemma 4 26B-A4B** (`mlx-community/gemma-4-26b-a4b-it-4bit`), served
locally by **`mlx_vlm.server`** — an OpenAI-compatible HTTP server — from a dedicated Python
venv that cerebro manages for you. cerebro **auto-launches and shuts down** that server.

The backend is **not hardcoded**. cerebro speaks two providers out of the box:

- `openai` — any OpenAI-compatible endpoint (MLX, LM Studio, llama.cpp `server`, Ollama's `/v1`).
- `ollama` — Ollama's native `/api/chat`.

Adding a model is a **config edit**, never a code change (see [Configuration](#configuration)).

## Requirements

- **Go 1.25+**
- **macOS (Apple Silicon)** for the default MLX backend, plus [`uv`](https://docs.astral.sh/uv/)
  to create the venv. (Other platforms work with a different provider/model in config.)
- The Gemma weights. If you've already pulled `mlx-community/gemma-4-26b-a4b-it-4bit` into your
  Hugging Face cache, the server reuses them; otherwise they download on first run (~15 GB).

## Install & run

```bash
# 1) Fetch Go dependencies
go mod tidy

# 2) One-time: create the dedicated venv and install MLX runtimes (mlx-lm + mlx-vlm)
make setup           # or: ./scripts/setup.sh   or:  go run ./cmd/cerebro --setup

# 3) Launch
go run ./cmd/cerebro  # or: make run, or build with `make build` then ./bin/cerebro
```

On first launch cerebro writes a default config, starts the backend (showing a "loading
model…" state), and drops you into the chat once it's ready.

Useful flags:

```bash
cerebro --config    # print config/data file locations
cerebro --setup     # (re)create the venv and install MLX runtimes
cerebro --version
```

## Usage

- **Enter** sends the prompt. **Ctrl+J** inserts a newline.
- **Esc** stops an in-progress response. **Ctrl+L** clears the conversation.
- **PgUp/PgDn** scroll the transcript. **Ctrl+H** toggles full help.
- **Ctrl+C**, **Ctrl+D**, or typing **`exit`**/**`quit`** exits cleanly.

### Slash commands

```
/help                     show help
/model list               list configured models
/model use <name>         switch active model
/header list              list header prompts
/header new <name>        create a header prompt (opens an editor)
/header edit <name>       edit a header prompt (opens an editor)
/header delete <name>     delete a header prompt
/header use <name>        activate a header prompt
/header none              disable the active header prompt
/clear                    clear the conversation
/exit                     quit
```

### Header prompts

A **header prompt** is a reusable instruction block (formatting rules, a role, coding-style
preferences) sent as a leading system message with every prompt until you change or disable it.
The active header prompt is always shown in the top bar (or `Header: none`). The header editor
opens for `/header new` and `/header edit`; save with **Ctrl+S**, cancel with **Esc**.

### Pasting large content

If you paste **more than 10 lines**, the input collapses the display to
`[Pasted Content X Lines]` while still sending the **full** text to the model. This relies on
your terminal's *bracketed paste* support (most modern terminals; see
[Limitations](#known-limitations)).

### Images

Type or paste an image **file path** (`.png`, `.jpg`, `.jpeg`, `.webp`, `.gif`) into the prompt.
It appears as an attachment chip and is sent to the model when the active model/provider supports
images. If the backend is text-only, cerebro warns and sends text only.

On **macOS** you can also paste an image directly from the clipboard with **Ctrl+V** (e.g. a
screenshot): cerebro saves it to a temp file and attaches it. Note that **Cmd+V** is handled by
the terminal, not the app, and won't paste image data — use **Ctrl+V**.

## Configuration

Files (XDG locations on macOS and Linux):

```
~/.config/cerebro/config.yaml           models, active/default model, server settings
~/.config/cerebro/header_prompts.yaml   header prompts + active selection
~/.local/share/cerebro/history/         session transcripts (JSON)
~/.local/share/cerebro/venv/            dedicated MLX venv (created by --setup)
~/.local/share/cerebro/server.log       managed-backend output
```

`config.yaml`:

```yaml
default_model: gemma4-26b
active_model: gemma4-26b

# Optional cerebro-managed backend. If base_url isn't already reachable and
# auto_launch is true, cerebro runs launch_command and waits for health_path.
server:
  auto_launch: true
  base_url: http://127.0.0.1:8080/v1
  health_path: /models
  startup_timeout_seconds: 240
  launch_command: ~/.local/share/cerebro/venv/bin/python -m mlx_vlm.server --model mlx-community/gemma-4-26b-a4b-it-4bit --port 8080

models:
  gemma4-26b:
    display_name: Gemma 4 26B-A4B
    provider: openai            # OpenAI-compatible (served by mlx_vlm.server)
    model: mlx-community/gemma-4-26b-a4b-it-4bit
    base_url: http://127.0.0.1:8080/v1
    supports_images: true
```

**Add another model** by adding a `models:` entry. Examples:

```yaml
  # An Ollama model (after `ollama pull llama3`):
  llama3:
    display_name: Llama 3
    provider: ollama
    model: llama3
    base_url: http://127.0.0.1:11434
    supports_images: false

  # Another MLX/OpenAI-compatible server you start yourself (set auto_launch:false):
  qwen:
    display_name: Qwen2.5
    provider: openai
    model: mlx-community/Qwen2.5-7B-Instruct-4bit
    base_url: http://127.0.0.1:8080/v1
    supports_images: false
```

Switch at runtime with `/model use <name>` (persisted to config).

## Architecture

```
cmd/cerebro/        entrypoint + flags (--setup, --config, --version)
internal/app/       wiring: config + stores + provider + manager -> TUI; backend setup
internal/config/    config.yaml load/save/validate; model -> provider spec
internal/prompts/   header-prompt CRUD + prompt assembly (pure)
internal/history/   session/saved conversation persistence (JSON)
internal/paste/     large-paste collapsing helper (pure)
internal/images/    image path detection + loading
internal/paths/     XDG file locations (dependency-free leaf)
internal/llm/       Provider interface, registry, server lifecycle manager
internal/llm/openai/  OpenAI-compatible streaming client (default / MLX)
internal/llm/ollama/  native Ollama client
internal/tui/       Bubble Tea model/update/view, styles, keys, slash commands
```

Providers self-register via `init()` (driver pattern); the app imports them for their side
effects. Implement `llm.Provider` and call `llm.Register` to add a backend kind.

## Testing

```bash
go test ./...
```

Covers config load/save/validation, header-prompt CRUD, prompt assembly, paste collapsing,
image detection/loading, provider selection, and the OpenAI/Ollama clients (via `httptest`).
The TUI and live model inference are validated by running the app.

## Known limitations

- **Live inference requires Apple Silicon + Metal** for the default MLX backend. Use a
  different provider/model in config on other platforms.
- **gemma4 vision** depends on `mlx-vlm` supporting the (new) `Gemma4ForConditionalGeneration`
  architecture. If your installed `mlx-vlm` doesn't yet, image requests return a clear error and
  text chat still works through the same server.
- **Paste collapsing** depends on terminal *bracketed paste*. Without it, pasted newlines are
  indistinguishable from rapid typing and won't collapse.
- **Clipboard image paste** (Ctrl+V) is macOS-only (uses `osascript`); on other platforms,
  attach images by typing/pasting a file path. Cmd+V is intercepted by the terminal and won't
  paste image data.
- The managed backend is for the default model. Switching to a model on a **different**
  `base_url` connects there but does not start that server for you (set `auto_launch: false`
  and start it yourself, or point `launch_command` at it).

## Troubleshooting

- **"Backend unavailable" / won't start** — run `cerebro --setup`, confirm
  `~/.local/share/cerebro/venv/bin/python` exists, and check `~/.local/share/cerebro/server.log`.
  Verify the port is free: `curl http://127.0.0.1:8080/v1/models`.
- **Slow first response** — the model loads into memory on first request; subsequent prompts
  are fast. Increase `server.startup_timeout_seconds` for very large models.
- **`uv` not found** — install from https://docs.astral.sh/uv/ (`brew install uv`).
- **Invalid config** — cerebro reports the parse error and the file path; fix or delete the
  file to regenerate defaults.
- **Using Ollama instead** — `ollama serve`, `ollama pull <model>`, add an `ollama` model entry,
  then `/model use <name>`.
