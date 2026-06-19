# Getting started

The first useful workflow is a local hook simulation:

```sh
go run ./cmd/compactor --help
tmp=$(mktemp -d)
printf '{"session_id":"demo","cwd":"%s","hook_event_name":"PreCompact","trigger":"manual"}\n' "$tmp" \
  | go run ./cmd/compactor hook claude precompact

printf '{"session_id":"demo","cwd":"%s","hook_event_name":"UserPromptSubmit","prompt":"continue"}\n' "$tmp" \
  | go run ./cmd/compactor hook claude inject
```

The hook workflow is:

1. Capture or receive the context that would otherwise be compacted.
2. Write durable documents with stable identifiers under `.compactor/sessions/<agent>/<session-id>/`.
3. Write an agent-readable `index.md` and `manifest.json`.
4. Emit compact references that can replace the original bulk context.
5. Reinject only the small `pending-context.md` capsule when the agent needs to continue after compaction.

Run the normal repo verification loop after edits:

```sh
dev/agent/check-fast
```
