# AGI OS — Go-To-Market Strategy

---

## 1. Core Challenge

AGI OS is a two-sided platform: it needs apps for agents to use, and agent users for app builders to care about. Classic cold-start problem.

The unlock: **the agent is the distribution channel**. Unlike traditional developer tools that require humans to discover, install, and learn them, AGI OS spreads through agent config files. One `agios init` sets up the project — every developer using an AI agent in that repo gets AGI OS automatically.

---

## 2. Distribution Mechanic: Agent Memory Files

`agios init` doesn't just create `agios.yaml` — it appends instructions to the project's agent memory file (`CLAUDE.md` or `AGENTS.md`). This means:

1. Developer runs `agios init` once
2. The agent memory file now tells every AI agent to use `agios`
3. Every developer on the team using Claude Code, Cursor, or any compatible agent gets the benefit immediately
4. No onboarding, no docs to read, no workflow changes

This is the key GTM mechanic. AGI OS bootstraps its own adoption through the agents it serves.

---

## 3. Phased Rollout

### Phase 1: Seed Supply

Don't wait for an ecosystem. Build 3–5 reference apps for the tools developers already live in:

- **gh** — GitHub PRs, issues, reviews
- **linear-cli** — task tracking
- **slack-cli** — messages, channels
- **notion** — docs and wikis
- **browser** — a built-in web browser for fetching, reading, and searching the web. Unlike the other apps, this ships with `agios` itself — no separate install needed. Gives agents a general-purpose escape hatch for any tool that doesn't have a dedicated app yet

These serve double duty: immediate value for users AND proof that the protocol works for app builders. Open source all of them.

### Phase 2: Atomic Use Case

The "home command" (unified notifications) is the wedge. Every developer has the problem of juggling multiple tabs to figure out what needs attention. Position AGI OS as:

> "Your agent checks GitHub, Linear, and Slack for you. One command. No tab switching."

This is demo-able, tweet-able, and a 30-second pitch. The notification feed is the landing page equivalent for a CLI product.

### Phase 3: Repo-by-Repo Adoption

Target teams already using AI coding agents heavily:

1. One developer runs `agios init` in the repo
2. Agent memory file is updated — every agent-using teammate benefits immediately
3. Viral within teams, zero friction

This is bottom-up, repo-by-repo adoption. No procurement, no sales calls.

### Phase 4: Protocol Evangelism

Once there are proof points (X teams, Y notifications processed, Z apps), publish the AIP spec formally and make it trivially easy to adopt:

- `agios scaffold` — generates a compliant app skeleton in Go/TS/Python/Rust
- A test harness that validates AIP compliance
- "Add AGI OS support to your CLI in 20 minutes" tutorial

### Phase 5: Ecosystem Flywheel

- Community app registry (GitHub topics / awesome-list initially, not a platform)
- "Works with AGI OS" badge for CLIs
- Highlight community-built apps

---

## 4. What NOT to Do

- **Don't build a marketplace** — AGI OS is a protocol, not a platform tax
- **Don't require an SDK** — the protocol is "output JSON to stdout with these fields." Zero dependencies is a feature
- **Don't target enterprises first** — this spreads developer-to-developer, repo-to-repo
- **Don't over-invest in a website** — the product is a CLI consumed by agents. The "website" is the `agios` command itself

---

## 5. Key Metrics

| Metric | What it measures |
| --- | --- |
| Apps available | How many AIP-compliant CLIs exist |
| Repos with `agios.yaml` | Adoption surface |
| Agent invocations / day | Actual usage (agents calling `agios`) |
| Notifications processed | Stickiness — are agents coming back? |

---

## 6. Positioning

**One-liner:** "The agent-native OS — your AI agent's interface to every developer tool."

The moat is the protocol, not the runtime. If AIP becomes the standard for how agents talk to tools, AGI OS wins regardless of which AI agent people use.
