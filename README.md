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
- **Lua plugin system** — drop a `.lua` file into `~/.config/termcode/plugins/`, restart TermCode, done. Plugins can add AI tools, slash commands, palette items, override theme colors, and contribute to the system prompt
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

### Plugin tools

You can extend the AI's toolset with in-process Go plugins. See
[Plugins](#plugins) below.

## Plugins

TermCode has a small plugin system based on **embedded Lua**. Plugins
are plain `.lua` files in `~/.config/termcode/plugins/`. Drop a file
in, restart TermCode, done — no Go, no rebuild, no subprocess, no
`.so`, works on every platform TermCode supports (Termux, Linux,
macOS, Windows).

Plugins can do five things:

1. **Register tools** the AI can call (same wire format as built-in tools).
2. **Register slash commands** users type into chat (e.g. `/hello`).
3. **Register palette items** that show up under `Ctrl+P`.
4. **Override theme colors** (any subset of the palette).
5. **Append fragments** to the AI's system prompt.

### Authoring a plugin

Create `~/.config/termcode/plugins/hello.lua`:

```lua
-- termcode is a global table provided by TermCode on load.

-- 1. Register a tool the AI can call.
termcode.register_tool(
    "hello_greet",                                -- name
    "Greet someone by name.",                     -- description
    "name (string, required) — person to greet", -- params
    function(params)
        local name = params.name or "world"
        return "Hello, " .. name .. "!"
    end
)

-- 2. Register a slash command. Users type "/hello <name>".
termcode.register_command(
    "/hello",
    "Say hi (optionally to a name).",
    function(argv)
        if argv == "" then return "Hi!" end
        return "Hi, " .. argv .. "!"
    end
)

-- 3. Register a palette item (Ctrl+P).
termcode.register_palette(
    "Hello — say hi",
    "Inserts a friendly greeting.",
    function() return "Hello from a Lua plugin!" end
)

-- 4. Override theme colors. Any subset is fine.
termcode.set_theme({ primary = "EC4899" }) -- hot pink

-- 5. Append a system-prompt fragment.
termcode.append_prompt("## hello active\nKeep replies under 3 sentences.")
```

Restart TermCode. The tool, command, palette item, theme, and prompt
fragment are immediately available — no rebuild of the TermCode
binary, no Go installed, just a `.lua` file.

### Plugin discovery

TermCode loads every `.lua` file in these directories, in order:

1. `~/.config/termcode/plugins/` (default)
2. Every directory listed in `$TERMCODE_PLUGIN_PATH` (colon-separated)

A broken file is logged to stderr and skipped; other plugins still
load.

### Listing loaded plugins

```bash
termcode plugin list
```

Example output:

```
1 plugin(s) loaded.
● hello
  Tools: 1
  Slash commands: 1
    /hello — Say hi (optionally to a name).
  Palette items: 1
    Hello — say hi — Inserts a friendly greeting from the hello plugin.
  Theme override: active
  System-prompt fragments: 1
```

### Caveats

- Plugin code runs in-process with the same privileges as TermCode.
  Don't load plugins you don't trust.
- Plugin tools share the same name space as built-in tools. Names
  like `read_file` are reserved.
- The Lua standard library is mostly available (string, math, table,
  etc.) but `os`, `io`, and `require` are not exposed to keep
  plugins sandboxed. (You can still build anything from the `string`
  library and pure Lua.)

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
│   ├── ai/                       # Provider clients + tool parser + context trim
│   ├── config/                   # Config + ProviderMeta + model limits
│   ├── session/                  # Session history → ~/.config/termcode/sessions/
│   ├── tools/                    # Built-in tools + plugin tool dispatch
│   └── luaplugin/               # Embedded Lua plugin loader (gopher-lua)
│   └── tui/                      # BubbleTea TUI (split into 14 files)
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
