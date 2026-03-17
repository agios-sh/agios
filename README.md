# AGI OS

[![CI](https://github.com/agios-sh/agios/actions/workflows/ci.yml/badge.svg)](https://github.com/agios-sh/agios/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/agios-sh/agios?label=release)](https://github.com/agios-sh/agios/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg)]()
[![Website](https://img.shields.io/badge/web-agios.sh-black.svg)](https://agios.sh)

The agent-native operating system.

AGI OS gives AI agents an ergonomic way to use external tools.
It defines and uses the **Agent Interface Protocol (AIP)** — a set of conventions for building CLI apps that are intuitive and efficient for agents — and provides a runtime that routes commands, manages output, and presents a unified surface across all configured apps.

```
agios                              # What's going on? (peek all apps)
agios <name>                       # Open a specific app
agios <name> <command> [args]      # Run any app command
```

## Install

**macOS / Linux**

```sh
curl -fsSL https://agios.sh/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://agios.sh/install.ps1 | iex
```

**From source**

```sh
git clone https://github.com/agios-sh/agios.git
cd agios
make install
```

## Quick start

```sh
# Initialize in your project
agios init

# See what's going on across all apps
agios

# Run built-in apps
agios tasks

# Check health of all connected apps
agios status
```

`agios init` creates an `agios.yaml` in your project root and appends instructions to your `AGENTS.md` / `CLAUDE.md` so AI agents discover it automatically.

## How it works

AGI OS is a single binary that acts as a router. It reads `agios.yaml` to know which apps are active, resolves each app binary on `$PATH`, and executes commands as subprocesses.

```yaml
# agios.yaml
apps: [agios-github]
```

```
Agent / Shell
    │
    ▼
AGI OS Runtime
    │  - Routes: agios <app> <cmd> → <binary> <cmd>
    │  - Validates output against AIP
    │  - Fetches peek snapshots from all apps
    │  - Converts output to TOON format (~40% token savings)
    │  - Truncates large values and redirect to temp files
    │  - Backgrounds long-running commands as jobs
    │
    ▼
┌─────────────────────────────────────┐
│    Any AIP-compatible CLI binary    │
└─────────────────────────────────────┘
```

### Built-in apps

AGI OS includes three built-in apps that don't require separate binaries:

- **`agios browser`** — headless browser automation via Chrome DevTools Protocol
- **`agios terminal`** — persistent terminal sessions with PTY support
- **`agios tasks`** — task tracking (local files)

### Key features

- **Peek** — `agios` with no arguments calls every app's `peek` command in parallel, showing a snapshot of current state across all apps. Like a dock where each app icon shows a badge.
- **TOON output** — app JSON is automatically converted to [TOON](https://toonformat.dev/) (Token-Oriented Object Notation) for ~40% token savings. Set `AGIOS_FORMAT=json` to get raw JSON.
- **Jobs** — commands that exceed the 5s timeout are automatically backgrounded. The agent gets a job ID and can check back with `agios jobs <id>`.
- **Progressive disclosure** — each response includes contextual `help` hints suggesting the next logical steps, not a full command listing.

## CLI reference

```
agios                           # Home — peek all apps
agios init                      # Create agios.yaml in current directory
agios add <name>                # Add an app
agios remove <name>             # Remove an app
agios status                    # Health check all apps
agios update                    # Check for and install updates
agios update check              # Check for updates without installing
agios help                      # Show available commands
agios jobs                      # List backgrounded jobs
agios jobs <id>                 # Get job status/result
agios <name> <command> [args]   # Route command to app
```

## Building an AIP app

Any CLI binary can become an AGI OS app. Implement three subcommands and follow the output standard:

### 1. `status` — identity and health

```sh
my-app status
```

```json
{
  "name": "my-app",
  "description": "Short description",
  "version": "0.1.0",
  "status": "ok",
  "commands": [
    { "name": "list", "description": "List items" },
    { "name": "get", "description": "Get item details" }
  ]
}
```

### 2. `help` — command reference

```sh
my-app help
```

```json
{
  "usage": "my-app <command> [args]",
  "commands": [
    {
      "name": "list",
      "description": "List items",
      "usage": "my-app list [--status open|closed]"
    },
    {
      "name": "create",
      "description": "Create item",
      "usage": "my-app create --title <title>"
    }
  ]
}
```

### 3. `peek` — state snapshot

```sh
my-app peek
```

```json
{ "ready": [{"id": "1", "title": "Fix auth bug"}] }
```

Return whatever fields best represent current state — keep it extremely concise and prefer actionable items over counters. Return `{}` if nothing to report.

### Output rules

- All stdout must be JSON (JSONL for progress updates)
- Include a `help` array with 2-3 contextual next steps
- Errors: `{"error": "...", "code": "ERROR_CODE", "help": [...]}`
- Warnings: include a `"warnings"` array (operation still succeeds)
- Stdin for content bodies, stdout for results, stderr for diagnostics

See [.agents/skills/agent-interface-protocol/SKILL.md](.agents/skills/agent-interface-protocol/SKILL.md) for the complete protocol reference and design principles.

## Development

```sh
make build              # Compile → ./agios
make test               # Run all tests
make test-race          # Tests with race detector
make lint               # gofmt + go vet
make check              # CI-equivalent: format check + vet + test
```

## License

[MIT](LICENSE)
