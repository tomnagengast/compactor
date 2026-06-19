# Agent context

Read [../../AGENTS.md](../../AGENTS.md) first. This page is the deeper map for agents that need more context than the root file should carry.

## Current state

The repo is a planning scaffold for a Go CLI. The only implemented behavior is help and version output in `cmd/compactor/main.go`.

The product concept is to preserve compacted agent context in local documents and replace active-window bulk with references. Claude and Codex are the first target adapters.

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

