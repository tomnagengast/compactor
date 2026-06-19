# Contributing

This project is early. Keep changes narrow and update docs alongside product decisions.

Use the wrapper commands for feedback:

```sh
dev/agent/check-fast
dev/agent/check-full
```

For code changes, prefer small, testable Go packages and keep agent-specific behavior behind clear boundaries. For docs changes, make sure `AGENTS.md`, `docs/agents/README.md`, and the user docs agree about commands and project status.

Do not commit local agent transcripts, generated compaction output, secrets, or private scratch files.

