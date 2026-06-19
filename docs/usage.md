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

Planned command areas:

- Generate Claude and Codex hook installer snippets.
- Parse transcripts into richer document shards.
- Resolve a reference back to its source document.
- Validate generated documents for drift, missing references, and unsafe paths.
