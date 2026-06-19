# compactor

**Turn agent compaction into progressive disclosure.** `compactor` is a planned CLI for converting dense Claude and Codex session context into durable documents, then leaving agents with references they can reopen only when needed.

[![Status](https://img.shields.io/badge/status-planning-lightgrey.svg)](#roadmap)
[![CI](https://github.com/tomnagengast/compactor/actions/workflows/ci.yml/badge.svg)](https://github.com/tomnagengast/compactor/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

## Current status

This repository is in planning scaffold state. The CLI can report help and version information, but compaction commands are intentionally not implemented until the Claude and Codex compaction model is mapped.

```sh
go run ./cmd/compactor --help
```

## Install

From source:

```sh
git clone https://github.com/tomnagengast/compactor
cd compactor
go build ./cmd/compactor
./compactor --help
```

Packaged releases and Homebrew installation will be added after the first usable workflow exists.

## Why this exists

Agent compaction usually trades detail for room in the context window. That keeps a session moving, but it can also flatten important decisions, tool outputs, and reasoning paths into a summary that the next turn cannot inspect.

`compactor` aims to preserve the detail outside the active context window. During compaction, it should write structured documents for the older conversation history and replace that bulk context with a compact map of references. The agent keeps a small index in view, then opens the underlying documents only when a task needs more detail.

This follows the same local-first family pattern as:

- [`scout`](https://github.com/tomnagengast/scout): map files before loading them.
- [`agent-memoryd`](https://github.com/tomnagengast/agent-memoryd): keep durable local memory available to agents.
- [`agent-insights`](https://github.com/tomnagengast/agent-insights): turn local agent sessions into useful reports.

## Early product boundaries

Initial design work is focused on Claude Code and Codex. The first useful version should understand each agent's compaction surface, produce files that are easy for agents to rediscover, and avoid depending on hosted services or private APIs.

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

1. Research Claude and Codex compaction behavior from official docs and observed local surfaces.
2. Design the document model for compacted history, references, and retrieval.
3. Implement a first manual workflow that converts captured context into files.
4. Add agent-specific adapters for Claude and Codex.
5. Add release packaging, Homebrew distribution, and richer smoke tests.

