This project was arhived because I can't continue upgrade it. I'm really sorry 😓
# TermCode

> AI coding assistant for the terminal — built for Termux on Android

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Android%20%7C%20Linux%20%7C%20macOS-lightgrey)]()
[![License](https://img.shields.io/badge/license-MIT-green)]()

TermCode is a terminal AI coding assistant written in Go. It runs natively in [Termux](https://termux.dev) on Android (no glibc, no Docker, no x86 required) and on any Linux/macOS machine.

Think of it as a lightweight alternative to OpenCode or Aider — compiled to a **single 10 MB static binary**.

```
┌─ TermCode [EN] ──────────────── ollama / qwen3-coder-next:cloud  📁 ~/myproject ─┐
│                                                                                    │
│  TermCode                                                                          │
│  I'll read the project structure first.                                            │
│ ┌──────────────────────────────────────────────────────────────────────────────┐   │
│ │ ⚡ list_files()                                                               │   │
│ └──────────────────────────────────────────────────────────────────────────────┘   │
│ ┌──────────────────────────────────────────────────────────────────────────────┐   │
│ │  cmd/  internal/  go.mod  Makefile                                           │   │
│ └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                    │
├────────────────────────────────────────────────────────────────────────────────────┤
│ ❯ │ Ask anything about your code...                                                │
├────────────────────────────────────────────────────────────────────────────────────┤
│ ✓ Ready — 4 msgs   last: 37.3 tok/s · 1519 tok   ctx 11% (30.5k/262.1k)          │
│ Enter send  Shift+Enter newline  Ctrl+P commands  /models switch  [EN] Ctrl+P.lang │
└────────────────────────────────────────────────────────────────────────────────────┘
```

## Features

- **Runs on Android** via Termux — native `arm64` binary, zero dependencies
- **Single ~10 MB binary** — no Node.js, no Python, no Docker
- **Streaming responses** — see the AI think in real time
- **Tool use** — AI can read, write, patch files, run shell commands, search the web, download files
- **Web search** — built-in DuckDuckGo search + page fetcher, no API key needed
- **Multi-provider** — Ollama (local + cloud), OpenAI, Anthropic, OpenRouter
- **Free cloud models** — works great with `glm-4.7:cloud` and `qwen3-coder-next:cloud` via Ollama (no GPU required)
- **Auto-detect context length** — reads real context window from Ollama `/api/show` (e.g. 262k for Qwen3-Coder-Next)
- **Smart context management** — auto-trims history to fit the model's window, shown in status bar
- **`<think>` tag filtering** — cleans reasoning traces from GLM/Qwen/DeepSeek, press `T` to expand
- **Interactive Q&A** — multi-select checkbox UI when AI asks clarifying questions
- **Language switcher** — EN/RU interface and AI response language (`Ctrl+P` → Switch Language)
- **Ollama model browser** — select and pull models from inside the TUI
- **Command palette** (`Ctrl+P`) — fuzzy-search all actions
- **Session history** — conversations saved to `~/.config/termcode/sessions/`
- **Token speed meter** — tok/s and context % in the status bar

## Why TermCode?

| | TermCode | OpenCode | Aider |
|---|---|---|---|
| Binary size | ~10 MB | ~80 MB (Node) | requires Python |
| Android/Termux | ✅ native | ❌ | ❌ |
| No GPU needed | ✅ (cloud models) | ✅ | ✅ |
| Offline capable | ✅ (Ollama local) | ✅ | ✅ |
| Web search | ✅ built-in | ❌ | ❌ |
| 256k context | ✅ auto-detect | manual | manual |

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

**Requirements:** Go 1.22+, CGO not required.

## Quick Start

```bash
# Start Ollama (if using local/cloud models)
ollama serve

# Run TermCode in your project directory
cd ~/myproject
termcode
```

On first run, TermCode creates `~/.config/termcode/config.json`.

### Recommended: Free Cloud Models (no GPU needed)

```bash
# Pull free cloud models via Ollama
ollama pull glm-4.7:cloud
ollama pull qwen3-coder-next:cloud   # 80B FP8, 262k context, free

# Select model inside TermCode with /models or Ctrl+P
```

### OpenAI / OpenRouter

Edit `~/.config/termcode/config.json`:

```json
{
  "active_provider": "openai",
  "providers": {
    "openai": {
      "base_url": "https://api.openai.com/v1",
      "api_key": "sk-...",
      "model": "gpt-4o-mini"
    },
    "openrouter": {
      "base_url": "https://openrouter.ai/api/v1",
      "api_key": "sk-or-...",
      "model": "qwen/qwen3-8b:free"
    }
  }
}
```

## Keybindings

| Key / Command | Action |
|---|---|
| `Enter` | Send message |
| `Shift+Enter` | New line in input |
| `Ctrl+P` | Open command palette |
| `Ctrl+S` | Save session |
| `Ctrl+C` | Quit |
| `T` | Toggle `<think>` block of last AI message |
| `↑↓` | Scroll chat |
| `/models` | Browse and switch Ollama models |
| `/pull <name>` | Download a model (`/pull qwen3:8b`) |

### Command Palette (`Ctrl+P`)

- **Switch Language** — EN ↔ RU (interface + AI responses)
- **New session** — clear history, start fresh
- **Load session** — browse and restore past conversations
- **Git status** — quick `git status` in chat
- **Go build / test** — run build or tests
- **Context info** — see token usage details
- **Clear screen** — wipe viewport

## Available Tools

TermCode gives the AI access to your project and the web:

| Tool | Description |
|---|---|
| `read_file` | Read any file in the project |
| `write_file` | Create or overwrite a file |
| `patch_file` | Replace a string in a file (preferred for small edits) |
| `list_files` | Show project file tree |
| `run_command` | Execute a shell command (30s timeout) |
| `web_search` | Search DuckDuckGo, no API key needed |
| `fetch_page` | Fetch and read a web page as plain text |
| `download_file` | Download a file from the internet (max 50 MB) |

### Example: AI searches the web

```
You: what's the latest Mindustry modding API for JS?

TermCode:
⚡ web_search("mindustry mod javascript API 2025")
⚡ fetch_page("https://github.com/Anuken/Mindustry/wiki/Modding")
...
Here's the current API reference...
```

### Interactive Q&A with multi-select

When the AI asks a clarifying question, TermCode shows a checkbox UI:

```
❓ Which components should I add?
  Space — select  ↑↓ — navigate  Enter — confirm

▶ ✓  Authentication
  ○  Database
  ✓  REST API
  ○  WebSocket

✏ [or type your own answer...]

  ✓ Selected: 2
```

## Recommended Models

| Model | Size | Notes |
|---|---|---|
| `qwen3-coder-next:cloud` | cloud | 80B FP8, 262k context, **free**, best quality |
| `glm-4.7:cloud` | cloud | Fast, free, good for everyday tasks |
| `qwen2.5-coder:7b` | 4.7 GB | Best local option for <8 GB RAM |
| `qwen2.5-coder:14b` | 9 GB | Better reasoning, needs 10+ GB RAM |
| `qwen3:8b` | 5.2 GB | General + coding |
| `deepseek-r1:7b` | 4.7 GB | Strong reasoning, has `<think>` blocks |

> **Note for Android/Termux:** Local models may crash on Mali GPU (Helio G88, etc.) due to a Vulkan driver bug in recent Ollama versions. Use free cloud models instead — they're faster anyway.

## Project Structure

```
termcode/
├── cmd/termcode/main.go          # Entry point
├── internal/
│   ├── ai/
│   │   ├── provider.go           # Ollama, OpenAI, Anthropic, OpenRouter + auto context detect
│   │   ├── toolparser.go         # Multi-format tool call parser (5 formats)
│   │   └── context.go            # Context window management & token trimming
│   ├── config/config.go          # Config + per-model token limits table
│   ├── session/session.go        # Session history → ~/.config/termcode/sessions/
│   ├── tools/tools.go            # All tools: files, shell, web search, download
│   └── tui/
│       ├── model.go              # BubbleTea Model — full UI & state machine
│       ├── highlight.go          # Syntax highlighting (pure Go, no deps)
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

PRs welcome. The codebase is intentionally small and readable:
- All TUI logic lives in `model.go` following the [BubbleTea](https://github.com/charmbracelet/bubbletea) Elm Architecture
- No CGO, no generated code, no build tags
- Compiles with a single `go build` command

## License

MIT
