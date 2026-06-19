# Contributing

This project is early. Keep changes narrow and update docs alongside product decisions.

Use the wrapper commands for feedback:

```sh
dev/agent/check-fast
dev/agent/check-full
```

For code changes, prefer small, testable Go packages and keep agent-specific behavior behind clear boundaries. For docs changes, make sure `AGENTS.md`, `docs/agents/README.md`, and the user docs agree about commands and project status.

Do not commit local agent transcripts, generated compaction output, secrets, or private scratch files.

## Sanitized fixtures

Transcript fixtures under `testdata` should preserve schema realism, not content realism. Raw Claude or Codex transcripts must be scrubbed outside the repo, manually reviewed, and only then copied into `testdata`.

Keep structural fields that parser adapters depend on, such as `message.content`, `toolUseResult`, `item`, `payload`, `event_msg`, `uuid`, `parentUuid`, `call_id`, `type`, and `name`. Replace names, emails, local paths, private URLs, hostnames, tokens, environment values, file contents, and long command output with neutral placeholders before committing.

Fixture tests should include leak checks for obvious private data. If a fixture needs a large tool output shape, use a short placeholder or an explicit redaction marker instead of the original output.
