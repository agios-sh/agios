# AIP Design Principles

AIP (Agent Interface Protocol) is a protocol for building apps that are ergonomic and intuitive for agents to use. This is the single reference for anyone building an app that works with AGI OS.

An AIP app is any CLI binary that follows these conventions. It can be written in any language.

---

## Required subcommands

Every app must implement three subcommands: `status`, `help`, and `peek`.

### `status`

How the OS discovers what the app can do and whether it's healthy.

```json
{
  "name": "my-app",
  "description": "Short description of what this app does",
  "version": "0.1.0",
  "status": "ok",
  "user": "baker@example.com",
  "commands": [
    { "name": "list", "description": "List items" },
    { "name": "get", "description": "Get item details" }
  ]
}
```

- `name`, `description`, `version` — app identity
- `status` — `ok` or `error`. If `error`, include an `error` field with details
- `user` — the authenticated user (if applicable)
- `commands` — what subcommands the app exposes

### `help`

How agents discover what an app can do and how to use it.

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

### `peek`

```
my-app peek
```

Returns a free-form JSON object representing the app's current state snapshot. Return an empty object `{}` if there's nothing to report.

```json
{
  "open": 5,
  "closed": 12,
  "recent": [
    {"id": "1", "title": "Fix auth bug", "status": "open"}
  ]
}
```

There is no fixed schema — each app decides what fields best represent its current state. The OS displays this data inline per app in the home command.

---

## Output standard

All app output to stdout MUST be JSON. Every response must be a JSON object.

### Errors

Errors use a structured format with `error`, `code`, and `help`:

```json
{
  "error": "Item not found",
  "code": "NOT_FOUND",
  "help": ["Run `agios my-app list` to see available items"]
}
```

Stderr is reserved for debug/diagnostic output that the OS ignores. All agent-facing output goes to stdout as JSON.

### Warnings

Non-fatal issues are reported as a top-level `warnings` array. The operation still succeeds.

```json
{
  "item": { "...": "..." },
  "warnings": ["'priority' field is not supported by this provider — ignored"]
}
```

### Long-running commands and progress

Output is JSONL (one JSON object per line). For most commands this is a single line. Long-running commands can emit progress lines before the final result:

```jsonl
{"progress": {"message": "Compiling dependencies...", "percent": 30}}
{"progress": {"message": "Running tests...", "percent": 75}}
{"items": [...], "help": ["..."]}
```

The last line (without a `progress` key) is the final result. The `percent` field is optional. If the app exceeds the OS timeout, the OS backgrounds it as a job automatically — apps don't need to handle this.

### Pagination

List responses that support pagination include `next` (opaque cursor) and `has_more` (boolean):

```json
{
  "items": ["..."],
  "next": "eyJ0IjoiMTcwNTMzMDgwMCJ9",
  "has_more": true
}
```

### Input

Commands that accept content bodies read from stdin. Apps should respect these env vars when set:

| Variable          | Behavior                            |
| ----------------- | ----------------------------------- |
| `AGIOS_FRESH=1`   | Bypass any local cache              |
| `AGIOS_VERBOSE=1` | Include additional detail in output |
| `AGIOS_QUIET=1`   | Return minimal output               |

### Token optimization

The OS automatically optimizes app output before returning it to the agent — apps don't need to do anything:

- **TOON conversion** — JSON is converted to [TOON](https://toonformat.dev/) for ~40% token savings. Apps always output JSON; the OS handles conversion.
- **Large value truncation** — strings exceeding 4096 chars are spilled to temp files and replaced with a file path reference.

---

## Design principles

Beyond the protocol requirements above, these principles make the difference between an app that technically works and one that agents love to use.

### No-args = dock view

Running `my-app` with no subcommand should show the app's **current state**, not static help text. Think of it like what a human would see after clicking an app icon in the macOS dock — you see what's happening right now.

- If the app manages a running process, show whether it's running and key details (PID, uptime, mode)
- If the app has open items (tabs, tasks, connections), list them
- If there's nothing active, show a brief status with hints on how to get started

The `help` subcommand exists for detailed usage docs. The no-args experience is about situational awareness.

### Idempotent operations

Don't error when the desired state already exists. If the agent asks to start something that's already running, or create something that already exists, acknowledge the current state and move on.

**Bad:** `{"error": "Already running (PID 1234)", "code": "ALREADY_RUNNING"}`

**Good:** `{"message": "Already running, using existing session", "pid": 1234}`

This is critical for agent workflows — agents may retry commands, run them defensively, or not track prior state. Idempotent responses let agents proceed without error-handling branches.

### Prefer success over errors

Reserve errors for situations where the user's intent genuinely cannot be satisfied. If the intent can be fulfilled (even partially or via an existing state), return a success response with context.

- "Start X" when X is running → success, "using existing X"
- "Create Y" when Y exists → success, "Y already exists", return it
- "Stop X" when X isn't running → success, "nothing to stop"

### Progressive disclosure

Every response's `help` array should suggest the **next logical steps** from the current state — not list every available command. The agent discovers the app's surface area organically by using it, not by reading a manual upfront.

**Bad:** Every response includes the same 15-line help listing all commands.

**Good:** Each response includes 2–3 contextual hints based on what just happened:

```json
// After "open"
"help": ["Run `app go <url>` to navigate", "Run `app quit` to stop"]

// After "go <url>"
"help": ["Run `app page` to see the page structure", "Run `app content` to extract text"]

// After "click <handle>"
"help": ["Run `app page` to see the updated page"]
```

This keeps token usage low and guides the agent through natural workflows. The `help` subcommand exists for the full reference — individual responses should be breadcrumbs, not encyclopedias.

#### Help hints must be copy-pasteable

If a command was invoked with a flag like `--source local`, every help hint in the response must also include `--source local`. An agent following a hint verbatim should hit the right resource — not silently target a different default.

**Bad:** After `create --source local`, help says `Run app get 1` — agent runs it against the default source (e.g., GitHub), not the local source where the task was just created.

**Good:** `Run app get 1 --source local`

The rule: if the resolved source (or any other disambiguating flag) differs from the default, append it to every hint.

#### Help should react to result data

Hints should reflect what the agent would logically want to do _next_ given the data they just received — not repeat a static list.

- After getting a **closed** task → suggest reopening, not closing again
- After an **empty list** → lead with `create`, not `get <id>` (there's nothing to get)
- After **updating status to closed** → suggest reopening as the next logical toggle
- In the **dock view** with no open tasks → lead with `create`, not `list`

**Bad:** Every `get` response suggests `--status closed` regardless of current status.

**Good:** Check the task's status and suggest the opposite action.

#### Error help should guide toward resolution

Error `help` arrays should suggest the specific command that addresses the problem — not a generic "run help". The agent just hit a wall; point them at the door.

| Error situation  | Bad hint       | Good hint                                    |
| ---------------- | -------------- | -------------------------------------------- |
| Task not found   | `Run app help` | `Run app list` to see available tasks        |
| List failed      | `Run app help` | `Run app status` to check source health      |
| Source not found | `Run app help` | `Run app status` to see all sources          |
| Update failed    | `Run app help` | `Run app get <id>` to verify the task exists |

### Expose user-facing state, not infrastructure

Responses should describe the state the agent cares about — not the implementation details behind it. If the app uses a background daemon, message queue, or database connection pool, the agent doesn't need to know. Surface the user-level concepts (sessions, tabs, items) and hide the plumbing.

**Bad:** `{"status": "running", "pid": 48231, "started": "2026-03-07T10:30:00Z", "socket": "/tmp/server.sock"}`

**Good:** `{"sessions": [...], "active_id": 2}`

The daemon PID, socket path, and uptime are only useful for debugging — not for the agent's next action. If the agent needs to know "are there sessions?", show the sessions. If there are none, show an empty list — don't say "server stopped" when what you mean is "no sessions".

### Validate inputs before calling external dependencies

Apps must validate all inputs **before** calling any external dependency — whether that's a CLI tool, a REST API, a database, or a third-party SDK. Never pass incomplete or invalid arguments through and let the dependency's error reach the agent.

External dependency errors are written for their own audience — CLI errors include usage text and flag listings, API errors return HTTP status codes and internal field names, database errors reference table schemas. None of this applies in the AIP context. An agent seeing raw dependency output will try to interpret and act on it, potentially hallucinating fixes or trying to interact with the underlying tool directly.

**Bad:** Pass user input straight through, let the dependency's error bubble up:

```json
{
  "error": "gh issue create: must provide `--title` and `--body` when not running interactively\n\nUsage: gh issue create [flags]\n\nFlags:\n  -b, --body string ..."
}
```

```json
{
  "error": "POST https://api.linear.app/graphql: {\"errors\":[{\"message\":\"Variable $input of type IssueCreateInput! was provided invalid value for title (Expected value to not be null)\"}]}"
}
```

**Good:** Validate at the app layer and speak the app's language:

```json
{
  "error": "Title is required to create a task",
  "code": "INVALID_ARGS",
  "help": ["Usage: `agios tasks create --title \"...\" [--body \"...\"]`"]
}
```

Rules:

1. **Validate required fields** at the app layer before calling any external dependency
2. **Provide sensible defaults** for fields the dependency requires but the user didn't provide (e.g., default `body` to empty string to avoid interactive prompts or null-value API errors)
3. **Translate errors** — if the dependency still fails, extract only the actionable meaning and discard implementation noise (usage text, flag listings, stack traces, raw HTTP responses, GraphQL error paths)

### Never expose internal dependency details in errors

Error messages returned to the agent should describe the problem in terms the agent can act on — using the app's own vocabulary, not the vocabulary of underlying dependencies. The agent interacts with `agios tasks`, not with `gh`, not with the GitHub API, not with a database. If an error leaks internal tool names, the agent may try to bypass the app and interact with the dependency directly.

**Bad:**

```json
{"error": "gh issue list: HTTP 401: Bad credentials"}
{"error": "POST /api/v1/issues returned 422: {\"message\":\"Validation failed\"}"}
{"error": "pq: relation \"tasks\" does not exist"}
```

**Good:**

```json
{"error": "GitHub authentication failed", "code": "AUTH_ERROR", "help": ["Run `gh auth login` to re-authenticate"]}
{"error": "Could not create task — the title is too long (max 256 characters)", "code": "VALIDATION_ERROR"}
{"error": "Task storage is not initialized", "code": "SETUP_ERROR", "help": ["Run `agios tasks setup` to initialize"]}
```

The agent should never need to know which CLI, API, database, or library powers the app. Errors should suggest next steps within the app's own interface.
