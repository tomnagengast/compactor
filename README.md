# compactor

**Turn agent compaction into progressive disclosure.** `compactor` is a CLI for converting dense Claude and Codex session context into durable local documents, then leaving agents with compact references they can reopen only when needed.

[![Status](https://img.shields.io/badge/status-alpha-yellow.svg)](#roadmap)
[![CI](https://github.com/tomnagengast/compactor/actions/workflows/ci.yml/badge.svg)](https://github.com/tomnagengast/compactor/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

## Current status

This repository has an alpha hook-driven implementation. The CLI can read Claude and Codex hook JSON, write local compaction documents under `.compactor/`, extract bounded agent-normalized transcript context, and emit a small reinjection capsule for prompt-cache-friendly progressive disclosure. Native Claude and Codex compaction have both been dogfooded with project hooks.

```sh
go run ./cmd/compactor --help
go run ./cmd/compactor hook claude precompact < hook-event.json
go run ./cmd/compactor resolve compactor://session/claude/session-1/index
go run ./cmd/compactor validate .compactor/sessions/claude/session-1
go run ./cmd/compactor hooks snippet claude --binary compactor
go run ./cmd/compactor hooks install claude --binary compactor
go run ./cmd/compactor hooks uninstall claude --binary compactor
```

## Install

From source:

```sh
git clone https://github.com/tomnagengast/compactor
cd compactor
go build ./cmd/compactor
./compactor --help
```

Install the latest release with Homebrew:

```sh
brew tap tomnagengast/tap
brew install --cask tomnagengast/tap/compactor-cli
compactor --version
```

## Why this exists

Agent compaction usually trades detail for room in the context window. That keeps a session moving, but it can also flatten important decisions, tool outputs, and reasoning paths into a summary that the next turn cannot inspect.

`compactor` aims to preserve the detail outside the active context window. During compaction, it should write structured documents for the older conversation history and replace that bulk context with a compact map of references. The agent keeps a small index in view, then opens the underlying documents only when a task needs more detail.

This follows the same local-first family pattern as:

- [`scout`](https://github.com/tomnagengast/scout): map files before loading them.
- [`agent-memoryd`](https://github.com/tomnagengast/agent-memoryd): keep durable local memory available to agents.
- [`agent-insights`](https://github.com/tomnagengast/agent-insights): turn local agent sessions into useful reports.

## Early product boundaries

Initial implementation work is focused on Claude Code and Codex. The current version understands each agent's compaction hook surface, produces files that are easy for agents to rediscover, and avoids hosted services or private APIs.

Non-goals for the first version:

- Replacing an agent's native compaction system.
- Storing secrets or raw private logs in a shared service.
- Optimizing for every agent runtime before Claude and Codex are understood.
- Building a large knowledge base before the document format proves useful.

## Docs

- [Docs index](./docs/README.md)
- [Install](./docs/install.md)
- [Getting started](./docs/getting-started.md)
- [Usage](./docs/usage.md)
- [Dogfood](./docs/dogfood.md)
- [Architecture](./docs/architecture.md)
- [Release](./docs/release.md)
- [Contributing](./docs/contributing.md)
- [Agent context](./docs/agents/README.md)

## Development

Use the agent wrappers for normal checks:

```sh
dev/agent/dev-status
dev/agent/check-fast
dev/agent/check-full
```

The wrappers are intentionally small while the repo is young. They will become the source of truth for local and CI feedback as implementation lands.

## Roadmap

1. Harden the real-agent dogfood harnesses and fixtures.
2. Improve extraction quality for decisions, tool results, and compact boundaries.
3. Add a resolver surface that agents can call directly, such as MCP or a tighter command workflow.
