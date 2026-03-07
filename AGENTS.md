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

**Go CLI with no framework** — command dispatch is a switch on `os.Args` in `main.go`. Reserved commands (`init`, `add`, `remove`, `status`, `help`, `jobs`) route to `cmd/` handlers; everything else is treated as an app name and routed through `cmd.RunApp()`.

### Package Responsibilities

- **`cmd/`** — Command handlers. Each command is a standalone function (e.g., `RunInit`, `RunAdd`, `RunStatus`).
- **`config/`** — Loads `agios.yaml` by walking up the directory tree (like git finds `.git/`). Config is a simple `apps:` list.
- **`runner/`** — Subprocess execution with 5s timeout, JSONL protocol parsing (progress lines + final result), background job management (`~/.agios/jobs/`), and binary path resolution.
- **`output/`** — Pipeline: normalize → truncate strings >4096 chars to temp files (`~/.agios/tmp/`) → convert to TOON or JSON format.
- **`peek/`** — Concurrent fetch of free-form state snapshots from all configured apps via errgroup.

### Key Protocols

**AIP (Agent Interface Protocol):** A protocol for building apps that are ergonomic and intuitive for agents to use. Apps are external binaries that accept `<app> <command> [args]` and output JSONL.
Lines with a `"progress"` key are progress updates; the last non-progress line is the result. All agios errors use structured JSON: `{"error": "...", "code": "ERROR_CODE", "help": [...]}`.

**TOON format:** Token-optimized output format (~40% token savings over JSON), used by default for agent-facing output. Controlled by `AGIOS_FORMAT` env var (`json` or `toon`).

### AIP Design Principles

When building or modifying AIP apps (including the built-in browser), follow these principles:

- **No-args = dock view** — `app` with no subcommand shows current state (running? what's open?), not static help. Like what a human would see after clicking an app icon in the mac os dock.
- **Idempotent operations** — don't error when the desired state already exists. "Start X" when X is running → return success with "using existing X", not an error.
- **Prefer success over errors** — reserve errors for when intent genuinely can't be satisfied. If the agent's goal can be met (even via existing state), return success with context.
- **Progressive disclosure** — each response's `help` array suggests a few next logical steps from the current state, not a full command listing. Agents discover the app by using it, not by reading a manual.

See `specs/aip-design-principles.md` for the complete AIP reference (protocol requirements + design principles).

## Code Style

- Standard `gofmt` formatting — CI enforces it
- `go vet` for static analysis
- No additional linters configured
- Platform-specific code uses build tags (`sysproc_unix.go`, `sysproc_windows.go`)
