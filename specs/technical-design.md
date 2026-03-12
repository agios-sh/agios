# AGI OS — Technical Design Document

## MVP Scope

This document describes the implementation of AGI OS MVP.
The MVP includes the core runtime, CLI surface, config management, notification aggregation, job management, and output pipeline.
The built-in `browser` app is excluded from MVP.

---

## 1. Language

**Go.** Single static binary, fast subprocess management, native cross-compilation to all target platforms, no runtime dependencies.

**Binary name:** `agios`

---

## 2. Release & Distribution

### 2.1 Versioning

[release-please](https://github.com/googleapis/release-please) automates version management using [Conventional Commits](https://www.conventionalcommits.org/). On merge to `main`, it creates a release PR; merging that PR creates a GitHub Release with a git tag.

### 2.2 Build Pipeline

GitHub Actions workflow (`.github/workflows/release.yml`) triggered when release-please creates a release. Two jobs:

1. **release-please** — creates the release and outputs version/tag
2. **build** — matrix job cross-compiling 6 targets, uploading archives to the GitHub Release

**Targets:**

| OS      | Arch  | Archive format |
| ------- | ----- | -------------- |
| darwin  | arm64 | `.tar.gz`      |
| darwin  | amd64 | `.tar.gz`      |
| linux   | arm64 | `.tar.gz`      |
| linux   | amd64 | `.tar.gz`      |
| windows | amd64 | `.zip`         |
| windows | arm64 | `.zip`         |

Archive naming: `agios_<os>_<arch>.<ext>`

Version is embedded via `-ldflags "-X main.version=${VERSION}"`. No CGo.

### 2.3 Install Scripts

Two install scripts checked into the main repo under `install/`:

- **macOS/Linux** (`install/install.sh`): Detects OS/arch via `uname`, fetches latest release archive from GitHub API, extracts binary to `/usr/local/bin`
- **Windows** (`install/install.ps1`): Detects arch, downloads `.zip` to `$USERPROFILE\.agios\bin`, adds to user `PATH` if needed

Install commands:

- macOS/Linux: `curl -fsSL https://agios.sh | bash`
- Windows: `powershell -c "irm agios.sh/install.ps1 | iex"`

**Hosting:** `agios.sh` is the apex domain pointed at GitHub Pages from this main (open source) repo. DNS setup:

- `agios.sh` → GitHub Pages (main repo, `docs/` folder)

### 2.4 CI Pipeline

Separate workflow (`.github/workflows/ci.yml`) runs on every push/PR to `main`: format check, vet, test, build.

---

## 3. Project Structure

```
agios/
├── main.go                  # Entrypoint, arg parsing, command dispatch
├── go.mod / go.sum
├── install/                 # Install scripts (install.sh, install.ps1)
├── cmd/                     # Command implementations
│   ├── root.go              # `agios` (home command)
│   ├── init.go              # `agios init`
│   ├── add.go / remove.go   # `agios add/remove <app>`
│   ├── status.go            # `agios status`
│   ├── help.go              # `agios help`
│   ├── jobs.go              # `agios jobs [id]`
│   └── app.go               # `agios <app> <command>` (catch-all router)
├── config/                  # Config loading and management
├── runner/                  # Subprocess exec, JSONL parsing, validation, jobs
├── output/                  # TOON conversion, large value truncation
├── notifications/           # Aggregation, seen state tracking
└── state/                   # Persistent state (~/.agios/state/)
```

---

## 4. CLI Parsing & Command Dispatch

Use Go's standard `os.Args` — no CLI framework needed. Reserved command names (`init`, `add`, `remove`, `status`, `help`, `jobs`) route to their handlers. Everything else is treated as an app name and routed to the app runner.

The app router loads `agios.yaml`, verifies the app is listed, resolves the binary on `$PATH`, execs the subprocess, validates output, runs it through the output pipeline, and writes to stdout.

---

## 5. Config Management

### `agios.yaml`

Lives in the project root. Lists active app binary names. The OS walks up from cwd to find the nearest `agios.yaml` (like git finds `.git/`).

### `agios init`

1. Creates `agios.yaml` with empty apps list
2. Appends AGI OS usage instructions to the project's agent memory file (`CLAUDE.md` or `AGENTS.md`). If neither exists, creates `AGENTS.md` and symlinks `CLAUDE.md` to it.

### `agios add/remove`

Adds or removes an app name from `agios.yaml`. When adding, first validate the app exists and satisfies the AIP contract.

---

## 6. App Execution Engine

### Subprocess Execution

Resolves app binary on `$PATH`, runs it with `exec.CommandContext` using a configurable timeout (default 5s). Captures stdout and stderr separately. Forwards stdin for content input. Passes through `AGIOS_FRESH`, `AGIOS_VERBOSE`, `AGIOS_QUIET` env vars.

### JSONL Parsing

App stdout is JSONL. Lines with a `progress` key are progress updates; the last non-progress line is the final result.
If parsing fails, return a protocol error that include the raw output.

---

## 7. Output Pipeline

When agios returns output to the agent, transform validated JSON before writing to stdout:

1. **Large value truncation** — string values exceeding 4096 chars are spilled to temp files in `~/.agios/tmp/` and replaced with a file path reference. Temp files cleaned up on a 1-hour TTL.
2. **TOON conversion** — JSON is converted to [TOON](https://toonformat.dev/) for ~40% token savings. Apps always output JSON; the OS handles conversion.

---

## 8. Job Management

When an app exceeds the timeout, the OS backgrounds the process, assigns a job ID (`j_<nanoid>`), and returns immediately with the latest progress and job ID.

Jobs are stored as JSON files in `~/.agios/jobs/`. `agios jobs` lists all jobs; `agios jobs <id>` returns progress or final result. Completed jobs are cleaned up after 24 hours.

---

## 9. Notification System

### Home Command (`agios`)

Runs with no arguments. For each app in `agios.yaml`, concurrently fetches `<binary> notifications --since <last_checked>` and `<binary> status`. Merges results into a unified view with per-app `unseen` count, `action_required` count, and a `peek` of the 3 most recent unseen notifications.

### Seen Tracking

Stored per-project in `~/.agios/state/<project-hash>/seen.json`. Running `agios` (home) does not mark notifications as seen — it's a peek. Running `agios <app> notifications` marks returned notifications as seen.

### Drill-down

`agios <app> notifications` runs the app's notifications command, passes output through the pipeline, and marks all returned IDs as seen.

---

## 10. Status Command

`agios status` concurrently runs `<binary> status` for each app. Reports health, version, and authenticated user per app. Missing binaries produce warnings.

---

## 11. State Directory Layout

```
~/.agios/
├── state/<project-hash>/    # Seen notifications, last_checked timestamps
├── jobs/                    # Backgrounded job state files
└── tmp/                     # Spilled large values (auto-cleaned)
```

---

## 12. Error Handling

All errors are JSON objects with `error`, `code`, and `help` fields. Error codes: `APP_NOT_CONFIGURED`, `BINARY_NOT_FOUND`, `INVALID_OUTPUT`, `NO_CONFIG`.

If an app exits non-zero with valid JSON containing an `error` field, pass it through. If stdout is empty or invalid, synthesize an error from stderr.

---

## 13. Dependencies

| Dependency                   | Purpose                |
| ---------------------------- | ---------------------- |
| `gopkg.in/yaml.v3`           | Parse `agios.yaml`     |
| `golang.org/x/sync/errgroup` | Parallel app execution |

No CLI framework. JSON handling uses stdlib.

---

## 14. Testing Strategy

**Unit tests:** Config parsing, JSONL parsing, output validation, TOON conversion, truncation, seen state.

**Integration tests:** A mock app binary in `testdata/mock-app/` that implements AIP. Configurable via env vars to simulate errors, slow responses, and bad output. Tests the full flow: init, routing, notifications, status, job backgrounding, and error cases.
