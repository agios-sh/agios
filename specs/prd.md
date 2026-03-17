# AGI OS

## The Agent-Native Operating System

### agios.sh

---

# 1. Vision

AGI OS is an operating system for AI agents. It provides:

1. **An ergonomics standard** — a protocol that helps developers make their apps agent-friendly
2. **A project config** — declares which apps are active, and warns when they're not available

---

# 2. Problem

Today's apps are designed for humans. GUIs, dashboards, web consoles — they assume a user with eyes, a mouse, and the ability to hold context across tabs. APIs exist, but they're designed for machines — backend integrations, not agentic workflows.

Agents need apps too. They need to check what's changed, read specs, manage tasks, send messages. But there's no standard for what an agent-friendly app looks like — how it should structure output, how it should communicate errors, how it should tell the agent what to do next.

We need a set of standards for building great apps for agents.

---

# 3. Core Thesis

> An operating system for agents.

Any CLI that follows the Agent Interface Protocol (section 4) can be an app. Apps can be built in any language and installed however you want (`brew`, `npm`, `pip`, `cargo`, a binary you compiled). List them in `agios.yaml` and they work:

```yaml
# agios.yaml
apps: [gh, linear-cli, my-custom-tool]
```

The OS looks up each binary on `$PATH`, validates output at runtime, fetches peek data, and presents a unified surface to the agent. If an app isn't installed, the OS warns on startup.

---

# 4. The Agent Interface Protocol

AIP is a protocol for building apps that are ergonomic and intuitive for agents to use. An app is any CLI binary that implements these conventions.

## 4.1 Status

Every app must implement a `status` subcommand. This is how the OS discovers what the app can do and whether it's healthy:

```
my-app status
```

```json
{
  "name": "my-app",
  "description": "Short description of what this app does",
  "version": "0.1.0",
  "status": "ok",
  "user": "baker@example.com",
  "commands": [
    {
      "name": "list",
      "description": "List items"
    },
    {
      "name": "get",
      "description": "Get item details"
    }
  ]
}
```

- `name`, `description`, `version` — app identity
- `status` — `ok` or `error`. If `error`, include an `error` field with details
- `user` — the authenticated user (if applicable)
- `commands` — what subcommands the app exposes

The OS calls `status` during `agios status` to report the health of all active apps.

## 4.2 Help

Every app must implement a `help` subcommand that describes available commands and usage:

```
my-app help
```

```json
{
  "usage": "my-app <command> [args]",
  "commands": [
    {
      "name": "list",
      "description": "List items",
      "usage": "my-app list [--status <status>]"
    },
    {
      "name": "get",
      "description": "Get item details",
      "usage": "my-app get <id>"
    },
    {
      "name": "create",
      "description": "Create a new item",
      "usage": "my-app create --title <title>"
    }
  ]
}
```

This is how agents discover what an app can do and how to use it.

## 4.3 Output Standard

All app output to stdout MUST be JSON. The OS enforces this shape:

### Required envelope

Every response must be a JSON object. The OS wraps or validates accordingly.

### Contextual help

Responses SHOULD include a `help` field — an array of natural-language strings describing follow-up actions available from the current state.

```json
{
  "items": ["..."],
  "help": [
    "Run `agios my-app get <id>` to view details",
    "Run `agios my-app list --status done` to see completed items"
  ]
}
```

Agents don't need to memorize command surfaces — the app tells them what to do next.

### Warnings

Non-fatal issues are reported as a top-level `warnings` array. The operation still succeeds.

```json
{
  "item": { "...": "..." },
  "warnings": ["'priority' field is not supported by this provider — ignored"]
}
```

### Errors

Errors are reported as a JSON object with `error` and optional `code`:

```json
{
  "error": "Item not found",
  "code": "NOT_FOUND",
  "help": ["Run `agios my-app list` to see available items"]
}
```

Stderr is reserved for debug/diagnostic output that the OS ignores. All agent-facing output goes to stdout as JSON.

### Token optimization

The OS automatically applies two optimizations to app output before returning it to the agent:

- **TOON conversion** — JSON output is converted to [TOON](https://toonformat.dev/) (Token-Oriented Object Notation), a compact encoding of the JSON data model designed for LLMs. This reduces token usage by ~40% with no information loss. Apps always output JSON; the OS handles conversion.
- **Large value truncation** — when any single string value exceeds a size threshold, the OS truncates it and writes the full output to a file. The truncated value is replaced with a path to the file so the agent can access the full content if needed.

### Long-running commands and progress

All app output to stdout is JSONL (one JSON object per line). For most commands this is just a single line — the final result. Long-running commands can optionally emit progress lines before the final result:

```jsonl
{"progress": {"message": "Compiling dependencies...", "percent": 30}}
{"progress": {"message": "Running tests...", "percent": 75}}
{"items": [...], "help": ["..."]}
```

The last line (without a `progress` key) is the final result. All preceding `progress` lines are intermediate updates. The `percent` field is optional.

**OS timeout behavior:** The OS enforces a response timeout. If an app hasn't returned its final result before the timeout, the OS:

1. Backgrounds the command as a **job**
2. Returns immediately to the agent with the latest progress (if any) and a job ID:

```json
{
  "job": "j_abc123",
  "app": "my-app",
  "status": "running",
  "progress": { "message": "Running tests...", "percent": 75 },
  "help": ["Run `agios jobs j_abc123` to check status"]
}
```

3. Continues capturing the app's output in the background

The agent can check on backgrounded jobs:

```
agios jobs                    # List all running/completed jobs
agios jobs <id>               # Get latest progress or final result
```

When the job completes, `agios jobs <id>` returns `"status": "completed"` with the final result.

This is transparent to apps — they just write JSONL to stdout. The OS decides when to background based on timing.

### Pagination

List responses that support pagination include `next` (opaque cursor) and `has_more` (boolean). The OS passes `--next <cursor>` to load subsequent pages.

```json
{
  "items": ["..."],
  "next": "eyJ0IjoiMTcwNTMzMDgwMCJ9",
  "has_more": true
}
```

## 4.4 Input Standard

### Content input

Commands that accept content bodies read from stdin:

```
echo "content" | agios <app> <command>
cat file.md | agios <app> <command>
```

### Environment variables

Apps should respect these env vars when set:

| Variable          | Behavior                            |
| ----------------- | ----------------------------------- |
| `AGIOS_FRESH=1`   | Bypass any local cache              |
| `AGIOS_VERBOSE=1` | Include additional detail in output |
| `AGIOS_QUIET=1`   | Return minimal output               |

---

# 5. Peek

A single command answers "what's going on?" across all apps.

## 5.1 How It Works

The OS uses a **pull model**. Every app must implement a `peek` subcommand:

```
my-app peek
```

Apps return a free-form JSON object representing their current state snapshot. Apps with nothing to report return an empty object `{}`. When an agent runs `agios` (the home command), the OS calls every app's `peek` command in parallel and presents the data inline per app.

## 5.2 Peek Schema

Peek returns a free-form JSON object. Each app decides what to show:

```json
{
  "ready": [
    {"id": "1", "title": "Fix auth bug"}
  ]
}
```

There is no fixed schema — apps choose whatever fields best represent their current state. Peek data should be extremely concise and prefer actionable items (e.g., ready tasks) over counters or history.

## 5.3 The Home Command

Running `agios` with no arguments shows all active apps with their peek data inline — like a dock where each app shows a snapshot of its current state.

```
agios
```

The OS calls `<app> peek` and `<app> status` for each active app in parallel, and returns:

```json
{
  "apps": [
    {
      "name": "gh",
      "summary": "GitHub CLI — issues, PRs, and repos",
      "peek": {
        "ready": [
          {"title": "Review requested on PR #789"}
        ]
      }
    },
    {
      "name": "linear",
      "summary": "Linear — issue tracking and project management",
      "peek": {
        "ready": [
          {"id": "LIN-42", "title": "Fix auth bug"}
        ]
      }
    },
    {
      "name": "slack",
      "summary": "Slack — team messaging and channels"
    }
  ],
  "help": [
    "Run `agios <app>` to see an app's current state",
    "Run `agios <app> help` to see all commands for an app"
  ]
}
```

The agent sees each app's current state at a glance and drills into the apps that matter:

```
agios <app>                              # App's dock view (current state)
agios <app> peek                         # Just the peek data
```

---

# 6. Config

## 6.1 `agios.yaml`

Lives in the repo root. Lists which apps are active for this project.

```yaml
# agios.yaml
apps: [gh, linear-cli, slack-cli]
```

- `apps` — list of executable names on `$PATH`
- Each app manages its own config and auth independently

On startup, the OS checks that each listed binary exists on `$PATH`. Missing apps produce a warning in the response — the OS continues with the apps that are available.

## 6.2 Project Initialization

```
agios init
```

Creates an `agios.yaml` with an empty `apps` list.

Additionally, `agios init` detects and appends to the project's agent memory file (`CLAUDE.md` or `AGENTS.md`). If neither exists, it creates `AGENTS.md` and symlink `CLAUDE.md` to it.
The appended instructions tell the agent to use `agios` as its interface to external tools:

```markdown
# AGI OS

This project uses AGI OS (agios) for agent-friendly access to external tools.

- Run `agios` to see all active apps and pending notifications
- Run `agios <app> <command>` to interact with a specific app
- Run `agios status` to check the health of all connected apps
- Always prefer `agios` over direct tool CLIs when available
```

This ensures that any AI agent working in the repo discovers AGI OS automatically — no human needs to remember to brief the agent.

---

# 7. Architecture

```
┌─────────────────────────────────────────────┐
│  AI Agent / Human / Shell                   │
│  agios [app] [command] [options]            │
└──────────────┬──────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────┐
│  AGI OS Runtime                             │
│  - Route: agios <app> <cmd> → <binary> <cmd>│
│  - Validate: check output against protocol  │
│  - Peek: fetch state snapshots from apps    │
│  - Scope: filter to project's active apps    │
└──────────────┬──────────────────────────────┘
               │  subprocess (exec)
               ▼
┌──────────┬──────────┬──────────┬────────────┐
│  gh      │ linear-  │ slack-   │  any CLI   │
│          │ cli      │ cli      │  binary    │
│  (Go)    │ (TS)     │ (Rust)   │  (any)     │
└──────────┴──────────┴──────────┴────────────┘
```

The OS is a single binary that routes commands to app binaries as subprocesses.

### Invocation flow

1. **Parse** — extract app name, command, flags from argv
2. **Resolve** — read `agios.yaml`, verify the app is listed, find it on `$PATH`
3. **Exec** — run `<binary> <command> [args]`, capture stdout
4. **Validate** — check the output is valid JSON, warn if `help` is missing
5. **Return** — pass the validated output to the caller

For the home command (`agios` with no args), step 3 runs in parallel across all apps.

---

# 8. CLI Surface

```
# Setup
agios init                      # Create agios.yaml
agios add <app>                 # Add an app to agios.yaml
agios remove <app>              # Remove an app from agios.yaml
agios status                    # Health check all apps in current project

# Home (the dock)
agios                           # Show all apps + notification counts

# Help
agios help                      # Show available commands and usage

# Jobs (long-running commands backgrounded by OS)
agios jobs                      # List all running/completed jobs
agios jobs <id>                 # Get latest progress or final result

# App commands (routed to the app binary)
agios <app> <command> [args]    # Any command the app exposes
agios <app> status              # App info + health
agios <app> peek                # App peek data (state snapshot)
```

---

# 9. Building Apps

Any CLI can become an AGI OS app. Minimum requirements:

### Step 1: Implement `status`

Your binary must respond to `my-app status` with app info (see section 4.1).

### Step 2: Implement `help`

Your binary must respond to `my-app help` with command descriptions (see section 4.2).

### Step 3: Implement `peek`

Your binary must respond to `my-app peek` with a JSON object representing your app's current state snapshot (see section 5.2). Return an empty object `{}` if there's nothing to report.

### Step 4: Follow the output standard

Return JSON on stdout. Include `help` arrays. Use `warnings` for non-fatal issues. Use the error format for errors.

### Step 5: Add to a project

Users add your app's executable name to their `agios.yaml`:

```yaml
apps: [my-app]
```

Your app now works with `agios my-app <command>`, appears in `agios status`, and shows its state in the home command.

Beyond these requirements, see [the AIP skill](../.agents/skills/agent-interface-protocol/SKILL.md) for principles on how apps should behave (idempotency, progressive disclosure, etc.).

---

# 11. Go-To-Market

See [gtm.md](gtm.md).

---

# 12. Open Design Questions

1. **App discovery** — Should there be a community registry or curated list of AGI OS-compatible apps? GitHub topics/awesome-list might be enough initially.
3. **Manifest evolution** — How to handle manifest schema changes as the protocol evolves? Versioning the protocol (`"protocol": 1`) seems prudent.
4. **Inter-app references** — Should the OS provide any facility for apps to reference each other's objects? (e.g., "this task has a linked PR"). Defer — apps can include cross-references in their own output if they want.

---

AGI OS — The Agent-Native Operating System.
agios.sh
