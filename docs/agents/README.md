# Agent context

Read [../../AGENTS.md](../../AGENTS.md) first. This page is the deeper map for agents that need more context than the root file should carry.

## Current state

The repo is an early Go CLI. It supports help/version output and hook commands for Claude and Codex compaction events.

The product concept is to preserve compacted agent context in local documents and replace active-window bulk with references. Claude and Codex are the first target adapters.

Core implementation map:

- `internal/hookio/`: parses hook JSON and emits hook-compatible JSON.
- `internal/snippet/`: generates copyable Claude and Codex hook config snippets.
- `internal/store/`: writes `.compactor/sessions/<agent>/<session-id>/`.
- `internal/capsule/`: keeps reinjected context small.
- `internal/cli/`: wires `compactor hook <agent> <phase>`.

## Normal loop

Run:

```sh
dev/agent/dev-status
dev/agent/check-fast
```

Use `dev/agent/check-full` for broader validation before finishing larger changes.

## Context maintenance

When product decisions land, update the smallest relevant surface:

- Root agent rules in `AGENTS.md`.
- Deeper agent context here.
- User-facing behavior in `README.md` and `docs/`.
- Deterministic checks in `dev/agent/`.

Do not let durable decisions live only in chat history.
