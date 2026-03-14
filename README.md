# TermCode

> AI coding assistant for the terminal — built for Termux on Android

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Android%20%7C%20Linux%20%7C%20macOS-lightgrey)]
[![License](https://img.shields.io/badge/license-MIT-green)]

TermCode is a terminal AI coding assistant written in Go. It runs natively in [Termux](https://termux.dev) on Android (no glibc, no Docker, no x86 required) and on any Linux/macOS machine.

Think of it as a lightweight, offline-capable alternative to tools like OpenCode or Aider — but compiled to a single static binary.

```
┌─ TermCode ──────────────────────── ollama / qwen2.5-coder:7b  📁 ~/myproject ─┐
│                                                                                 │
│  TermCode                                                                       │
│  I'll read the project structure first.                                         │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ ⚡ list_files()                                                              │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │  cmd/  internal/  go.mod  Makefile                                          │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ❯ │ Ask anything about your code...                                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ✓ Ready — 4 messages          last: 8.3 tok/s · 92 tok   ctx 3% (1.2k/32.0k) │
│ Enter send  Shift+Enter newline  Ctrl+P commands  /models switch  Ctrl+C quit  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Features

- **Runs on Android** via Termux — native `arm64` binary, zero dependencies
- **Streaming responses** — see the AI think in real time
- **Tool use** — AI can read, write, patch files and run shell commands
- **Multi-provider** — Ollama (local), OpenAI, Anthropic, OpenRouter
- **Smart context management** — auto-trims history to fit model's context window
- **`<think>` tag filtering** — cleans up reasoning traces from GLM/Qwen/DeepSeek
- **Ollama model browser** — select and pull models from inside the TUI
- **Command palette** (`Ctrl+P`) — fuzzy-search all actions
- **Session history** — conversations saved to `~/.config/termcode/sessions/`
- **Token speed meter** — shows tok/s and context usage in the status bar

## Installation

### Termux (Android)

```bash
# Install Go
pkg install golang git

# Clone and build
git clone https://github.com/AITechnologyDev/termcode
cd termcode
bash build-termux.sh

# Run
~/bin/termcode
```

### Linux / macOS

```bash
git clone https://github.com/AITechnologyDev/termcode
cd termcode
make build        # binary → ./termcode
# or
make install      # installs to ~/bin/termcode
```

**Requirements:** Go 1.22+, no CGO needed.

## Setup

On first run, TermCode creates `~/.config/termcode/config.json`.

### Ollama (recommended for offline use)

```bash
# Install Ollama, then pull a model
ollama pull qwen2.5-coder:7b

# Run TermCode — it will auto-detect Ollama
termcode
```

### OpenAI / OpenRouter

```bash
termcode config set-provider openai --api-key sk-...
termcode config set-provider openrouter --api-key sk-or-...
```

### Manual config edit

```json
{
  "active_provider": "ollama",
  "providers": {
    "ollama": {
      "base_url": "http://127.0.0.1:11434",
      "model": "qwen2.5-coder:7b"
    },
    "openai": {
      "base_url": "https://api.openai.com/v1",
      "api_key": "sk-...",
      "model": "gpt-4o-mini"
    }
  }
}
```

## Usage

| Key / Command | Action |
|---|---|
| `Enter` | Send message |
| `Shift+Enter` | New line in input |
| `Ctrl+P` | Open command palette |
| `Ctrl+S` | Save session |
| `Ctrl+C` | Quit |
| `/models` | Browse and switch Ollama models |
| `/pull <name>` | Download a model (e.g. `/pull qwen3:8b`) |
| `↑↓` | Scroll chat |

### Command Palette (Ctrl+P)

Press `Ctrl+P` to open the palette. Start typing to fuzzy-search:

- **New session** — clear history, start fresh
- **Git status** — quick `git status` output in chat
- **Go build / test** — run build or tests via AI
- **Context info** — see token usage
- **Clear screen** — wipe viewport

### Tool Use

TermCode gives the AI access to your project files:

```
You: refactor the error handling in internal/tools/tools.go

TermCode: I'll read the file first.
⚡ read_file(path=internal/tools/tools.go)
...
⚡ patch_file(path=internal/tools/tools.go, old_str=..., new_str=...)
Done. I wrapped the error in fmt.Errorf with context.
```

Available tools: `read_file`, `write_file`, `patch_file`, `list_files`, `run_command`.

## Recommended Models

| Model | Size | Good for |
|---|---|---|
| `qwen2.5-coder:7b` | 4.7 GB | Everyday coding, fast |
| `qwen2.5-coder:14b` | 9 GB | Better reasoning |
| `qwen3:8b` | 5.2 GB | General + coding |
| `glm-4.7:cloud` | cloud | Fast cloud via Ollama |
| `deepseek-coder` | 4.7 GB | Code generation |
| `codestral` | 12 GB | Large context coding |

## Project Structure

```
termcode/
├── cmd/termcode/main.go          # CLI entry point (cobra)
├── internal/
│   ├── ai/
│   │   ├── provider.go           # Ollama, OpenAI, Anthropic, OpenRouter streaming
│   │   ├── toolparser.go         # Multi-format tool call parser
│   │   └── context.go            # Context window management & trimming
│   ├── config/config.go          # Config + per-model token limits
│   ├── session/session.go        # Session history → ~/.config/termcode/sessions/
│   ├── tools/tools.go            # read_file, write_file, patch_file, list_files, run_command
│   └── tui/
│       ├── model.go              # BubbleTea Model — all UI logic
│       ├── runner.go             # tea.NewProgram launcher
│       └── styles.go             # lipgloss dark theme
├── go.mod
├── Makefile
└── build-termux.sh               # One-shot Termux build script
```

## Building from Source

```bash
# Standard build
go build -o termcode ./cmd/termcode

# Termux / Android (static, no CGO)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -ldflags="-s -w" -o termcode ./cmd/termcode

# Use the helper script in Termux
bash build-termux.sh
```

## Contributing

PRs welcome. The codebase is intentionally small — the entire TUI is in one file (`model.go`) following the Elm Architecture pattern from [BubbleTea](https://github.com/charmbracelet/bubbletea).

## License

MIT
