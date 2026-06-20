# Dogfood workflow

Use this workflow to test `compactor` with real Claude or Codex compaction without risking user-level hook settings or committing sensitive artifacts.

## Safety rules

Start with manual hook simulation. Do not install hooks with `--write` until the dry-run output has been reviewed.

Use project-scope hooks in a sacrificial repo before user-scope hooks. Treat `.compactor/sessions/` as sensitive because it can contain bounded transcript extracts, local paths, and scoped tool output documents. Do not commit `.compactor/`, `.claude/`, `.codex/`, raw transcripts, hook payloads, or generated dogfood artifacts.

Hook failures are expected to continue native compaction with warnings, but dogfood runs should still inspect warnings closely.

## Setup

```sh
PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
dev/agent/dev-status
dev/agent/check-fast
go build -o /tmp/compactor-dogfood ./cmd/compactor
```

Generate snippets and dry-run installs:

```sh
/tmp/compactor-dogfood hooks snippet claude --binary /tmp/compactor-dogfood
/tmp/compactor-dogfood hooks snippet codex --binary /tmp/compactor-dogfood
/tmp/compactor-dogfood hooks install claude --binary /tmp/compactor-dogfood --dry-run
/tmp/compactor-dogfood hooks install codex --binary /tmp/compactor-dogfood --dry-run
```

Review diagnostics before using `--write`.

## Manual simulation

Run fake hook payloads before installing hooks:

```sh
printf '{"session_id":"dogfood-claude","cwd":"%s","hook_event_name":"PreCompact","trigger":"manual"}\n' "$PWD" \
  | /tmp/compactor-dogfood hook claude precompact

/tmp/compactor-dogfood validate .compactor/sessions/claude/dogfood-claude
/tmp/compactor-dogfood resolve compactor://session/claude/dogfood-claude/index
```

Repeat with `codex`.

## Real Claude run

After reviewing project `.claude/settings.json`, trigger `/compact` in a sacrificial Claude session. Claude can provide a native compact summary through `PostCompact`, so check for:

- `summaries/native.md`
- event log entries for `PreCompact` and `PostCompact`
- bounded transcript extracts in `timeline.md`
- references, not summaries, in `pending-context.md`

## Real Codex run

The most reliable Codex dogfood path is the native app-server protocol, not TUI key driving. The repo includes a wrapper that builds the current checkout, temporarily installs project Codex hooks pointing at that build, starts `codex app-server --listen stdio://`, sends three turns, calls `thread/compact/start`, validates generated docs, submits one follow-up prompt, and restores the prior `.codex/hooks.json` on exit:

```sh
dev/agent/dogfood-codex-appserver
```

This is a real Codex/model run. It writes ignored local artifacts under `.codex/`, `.compactor/`, and `$CODEX_HOME/sessions/`, and it can consume model quota.

The verified Codex event shape is:

- `PreCompact` hook starts and completes for the compact turn.
- Codex emits an `item/completed` notification whose item type is `contextCompaction`.
- `PostCompact` hook starts and completes for the same compact turn.
- The compact turn completes.
- A later `UserPromptSubmit` hook can inject the `pending-context.md` capsule; a no-tools follow-up prompt should see a `compactor://session/codex/...` ref in context.

In the verified app-server path, Codex did not emit `thread/compacted`. Dogfood automation should wait for the compact turn completion plus the `contextCompaction` item and `PostCompact`, not only for `thread/compacted`.

The Codex hook payload has been observed to provide the session/thread id, cwd, transcript path, trigger, and turn id. Codex has not been observed to provide native compact summary text, so Codex uses precompact transcript-derived artifacts plus the postcompact event metadata.

## Validation checklist

For each generated session:

```sh
/tmp/compactor-dogfood validate .compactor/sessions/<agent>/<session-id>
/tmp/compactor-dogfood resolve compactor://session/<agent>/<session-id>/index
/tmp/compactor-dogfood resolve compactor://session/<agent>/<session-id>/decisions
/tmp/compactor-dogfood resolve compactor://session/<agent>/<session-id>/tool-results
```

Confirm:

- `pending-context.md` stays small and contains refs/paths, not full summaries.
- `timeline.md` records hook phase, event log, transcript path, transcript bytes, and bounded extracts.
- `decisions.md` has useful candidate sections.
- `tool-results.md` references tool calls/results and any large output documents.
- `manifest.json` has `raw_transcript_stored=false`.
- Large output docs resolve and validate when present.

## Observations to capture

Record these in a follow-up issue, PR note, or sanitized fixture plan:

- actual Claude and Codex hook payload fields
- whether `transcript_path` is consistently present
- whether Codex ever provides compact summary data
- whether `UserPromptSubmit` injection is too noisy
- whether future Codex versions emit `thread/compacted` in addition to `contextCompaction`
- prompt-cache churn, startup warnings, or hook config incompatibilities
- whether bounded extraction is sufficient to recover prior task context
