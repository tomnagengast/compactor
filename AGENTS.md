# Agent Instructions

## Project Context

`compactor` is a planned local-first CLI that helps Claude and Codex turn compaction into progressive disclosure. The intended workflow is to convert older message history and surrounding context into durable documents, then keep compact references in the active context window so agents can reopen detail only when needed.

The current implementation is only a planning scaffold. The CLI entry point is `cmd/compactor/main.go`, and product design notes live under `docs/`.

## Repository Map

- `cmd/compactor/`: CLI entry point.
- `docs/`: user and design docs.
- `docs/agents/README.md`: deeper context for coding agents.
- `dev/agent/`: non-interactive wrappers for status and checks.
- `.github/`: issue, PR, dependency, and CI configuration.

## Development Workflow

Prefer the wrapper commands because CI uses the same surface:

```sh
dev/agent/dev-status
dev/agent/check-fast
dev/agent/check-full
```

For focused Go work, `go test ./...` and `go build ./cmd/compactor` are acceptable, but update the wrappers when the normal feedback loop changes.

## Safety Rails

Treat local agent transcripts, compaction inputs, and generated history documents as potentially sensitive. Do not commit captured sessions, raw logs, secrets, local config dumps, or generated scratch artifacts unless the user explicitly asks and the file is intentionally sanitized.

Do not add live-service dependencies, hosted storage, release publishing, or destructive filesystem behavior without an explicit product decision. Keep the first implementation local and inspectable.

## Done Criteria

For documentation-only changes, read the changed files back and run `dev/agent/check-fast`.

For code changes, run `dev/agent/check-fast`. Run `dev/agent/check-full` before landing broader CLI, release, or workflow changes.

When commands, architecture, ownership, or feedback loops change, update this file or `docs/agents/README.md` in the same change.

