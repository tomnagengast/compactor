# Usage

Current command surface:

```sh
compactor --help
compactor --version
compactor hook claude precompact
compactor hook claude postcompact
compactor hook claude inject
compactor hook codex precompact
compactor hook codex postcompact
compactor hook codex inject
compactor hooks snippet claude
compactor hooks snippet codex
```

Each hook command reads one JSON hook payload from stdin. `precompact` and `postcompact` write local session documents and return hook-compatible JSON that allows native processing to continue. `inject` reads the pending capsule and returns hook-compatible JSON with `additionalContext`.

After the agent and phase are valid, hook commands are best-effort: decode or write failures are returned as hook-compatible warnings with `continue: true` so native compaction can proceed.

Generated session documents live under:

```text
.compactor/sessions/<agent>/<session-id>/
```

The first document set is:

- `manifest.json`: machine-readable metadata and document refs.
- `index.md`: agent-readable map.
- `timeline.md`: compaction event and transcript metadata.
- `decisions.md`: placeholder for extracted decisions and constraints.
- `summaries/native.md`: native compact summary when the hook payload provides one.
- `pending-context.md`: small reinjection capsule.

Example:

```sh
printf '{"session_id":"demo","cwd":"%s","hook_event_name":"PreCompact","trigger":"manual"}\n' "$PWD" \
  | compactor hook claude precompact
```

## Hook snippets

Use `hooks snippet` to generate copyable JSON config without modifying any settings files:

```sh
compactor hooks snippet claude --binary /absolute/path/to/compactor
compactor hooks snippet codex --binary /absolute/path/to/compactor
```

If `--binary` is omitted, `compactor` uses the currently running executable path. When running through `go run`, pass `--binary compactor` or a built binary path so snippets do not point at a temporary Go build cache.

The snippet wires:

- `PreCompact` to `compactor hook <agent> precompact`.
- `PostCompact` to `compactor hook <agent> postcompact`.
- `SessionStart` with matcher `compact` to `compactor hook <agent> inject`.
- `UserPromptSubmit` to `compactor hook <agent> inject`.

Planned command areas:

- Write hook snippets into agent settings after review.
- Parse transcripts into richer document shards.
- Resolve a reference back to its source document.
- Validate generated documents for drift, missing references, and unsafe paths.
