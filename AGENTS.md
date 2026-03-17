# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Build & Development Commands

```bash
make build            # Compile binary → ./agios
make run ARGS="status" # Build and run with args
make test             # Run all tests
make test-v           # Verbose test output
make test-race        # Tests with race detector
make lint             # Format (gofmt -w) + vet
make check            # CI-equivalent: format check + vet + test
make mock-app         # Build mock binary for integration tests
make install          # Install to $GOPATH/bin
```

Run a single test: `go test -v -run TestName ./path/to/package/`

Integration tests (`integration_test.go`) compile both `agios` and `testdata/mock-app/` into temp dirs, so they're self-contained — just run `go test -v -run TestIntegration .`

## Architecture

**Go CLI with no framework** — command dispatch is a switch on `os.Args` in `main.go`. Reserved commands (`init`, `add`, `remove`, `status`, `help`, `jobs`, `update`) route to `cmd/` handlers; everything else is treated as an app name and routed through `cmd.RunApp()`.

### Package Responsibilities

- **`cmd/`** — Command handlers. Each command is a standalone function (e.g., `RunInit`, `RunAdd`, `RunStatus`).
- **`config/`** — Loads `agios.yaml` by walking up the directory tree (like git finds `.git/`). Config is a simple `apps:` list.
- **`runner/`** — Subprocess execution with 5s timeout, JSONL protocol parsing (progress lines + final result), background job management (`~/.agios/jobs/`), and binary path resolution.
- **`output/`** — Pipeline: normalize → truncate strings >4096 chars to temp files (`~/.agios/tmp/`) → convert to TOON or JSON format.
- **`peek/`** — Concurrent fetch of free-form state snapshots from all configured apps via errgroup.
- **`updater/`** — Self-update mechanism: checks GitHub releases for newer versions, downloads archives with SHA-256 checksum verification, and atomically replaces the binary. Caches check results in `~/.agios/update-check.json` (24h TTL) and supports background update checks.

### Key Protocols

**AIP (Agent Interface Protocol):** A protocol for building apps that are ergonomic and intuitive for agents to use. Apps are external binaries that accept `<app> <command> [args]` and output JSONL.
Lines with a `"progress"` key are progress updates; the last non-progress line is the result. All agios errors use structured JSON: `{"error": "...", "code": "ERROR_CODE", "help": [...]}`.

**TOON format:** Token-optimized output format (~40% token savings over JSON), used by default for agent-facing output. Controlled by `AGIOS_FORMAT` env var (`json` or `toon`).

### AIP Design Principles

The `/aip-design-principles` skill auto-loads the full AIP spec when building or modifying AIP apps. It covers the protocol (required subcommands, output format, errors, progress, pagination) and design principles (idempotency, progressive disclosure, input validation, error translation).

## Code Style

- Standard `gofmt` formatting — CI enforces it
- `go vet` for static analysis
- No additional linters configured
- Platform-specific code uses build tags (`sysproc_unix.go`, `sysproc_windows.go`)
