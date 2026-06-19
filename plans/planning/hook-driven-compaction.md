# Hook-driven compaction implementation plan

> updated: 2026-06-19T13:44:00-07:00

## Goal

Build `compactor` as a local-first hook companion for Claude and Codex. On compaction, it preserves the detail that would otherwise leave the active context window as local documents, then reinjects only a small progressive-disclosure capsule that tells the agent where to look if it needs prior detail.

The first implementation should be deterministic, cache-aware, and safe to run from agent lifecycle hooks.

## Decisions

Use hooks for both Claude and Codex.

Use `PreCompact` to snapshot context and generate compact docs before native compaction changes the conversation shape.

Use `PostCompact` to record native compaction metadata and mark a pending capsule for reinjection. Claude can capture `compact_summary` directly. Codex currently documents only trigger and turn metadata for `PostCompact`, so Codex should rely on precompact artifacts plus transcript snapshots.

Use `SessionStart(source=compact)` or `UserPromptSubmit` to inject context back into the window. Do not depend on `PostCompact` alone for reinjection. The injection should be a small reference capsule, not the full generated summaries.

Treat prompt caching as a first-class constraint. Do not mutate root instructions, hook config, tool definitions, or MCP config during a session. Keep hook output deterministic, sorted, and tiny. Put dynamic detail in files, not early prompt layers.

## Architecture

Core package:

- Parse hook JSON from stdin into a small normalized event model.
- Resolve agent, session id, cwd, transcript path, trigger, turn id, and compact summary.
- Create `.compactor/sessions/<agent>/<session-id>/`.
- Write deterministic files:
  - `manifest.json`: machine-readable metadata, paths, timestamps, event history, source transcript path, trigger, and document ids.
  - `index.md`: human/agent-readable entry point.
  - `timeline.md`: chronological source snapshot or transcript-derived notes.
  - `decisions.md`: initially a placeholder section with extraction notes; later LLM-assisted or rule-assisted extraction.
  - `summaries/native.md`: native compact summary when available.
  - `pending-context.md`: tiny reinjection capsule.
- Never write raw transcript content into git-tracked paths unless the user explicitly configures that. The default `.compactor/` directory should be gitignored.

CLI surface:

- `compactor hook claude precompact`
- `compactor hook claude postcompact`
- `compactor hook claude inject`
- `compactor hook codex precompact`
- `compactor hook codex postcompact`
- `compactor hook codex inject`
- `compactor --help`
- `compactor --version`

`precompact` behavior:

- Read hook JSON from stdin.
- Refuse to fail native compaction for ordinary parse or write issues unless `--strict` is added later.
- Write or refresh the session docs.
- If a transcript path is present, copy bounded metadata into `timeline.md`; do not duplicate raw logs by default.
- Write `pending-context.md`.
- Emit hook-compatible JSON with no large `additionalContext`.

`postcompact` behavior:

- Read hook JSON from stdin.
- Append event metadata to `manifest.json`.
- For Claude, write `summaries/native.md` from `compact_summary`.
- Refresh `index.md` and `pending-context.md`.
- Emit hook-compatible JSON that continues normal processing.

`inject` behavior:

- Read hook JSON from stdin.
- Locate the session directory and pending capsule.
- Emit agent-specific hook JSON that adds the capsule as extra context:
  - Claude: `hookSpecificOutput.additionalContext`.
  - Codex: `hookSpecificOutput.additionalContext` for `UserPromptSubmit`, or common `systemMessage` only when additional context is not supported for the event.
- Keep the capsule under a conservative size limit, ideally 1-2 KB.

## Data model

Manifest fields:

- `version`
- `agent`
- `session_id`
- `cwd`
- `session_dir`
- `transcript_path`
- `created_at`
- `updated_at`
- `events`
- `documents`
- `pending_context_path`
- `privacy`

Event fields:

- `event_name`
- `trigger`
- `turn_id`
- `received_at`
- `source`

Document fields:

- `id`
- `path`
- `kind`
- `summary`
- `retrieval_hint`

## Cache-aware output rules

- Stable ordering everywhere.
- Stable headings and ids.
- No random ids when session id is available.
- No wall-clock timestamps in the injected capsule.
- No large summaries in the injected capsule.
- No raw transcript excerpts in the injected capsule.
- Keep changing content at the end of hook output, never in persistent instructions.

## Privacy and safety

- Default output path is `.compactor/`, and `.compactor/` must be ignored by git.
- Do not commit generated session documents.
- Store transcript path references, not raw full transcripts, by default.
- Add a later `--include-raw-transcript` or `capture.raw_transcript = true` only after explicit design.
- Redaction is a later feature, but the manifest should already have a `privacy` block so the format has a place for it.

## First implementation slice

1. Add `.compactor/` and `insights/` to `.gitignore`.
2. Replace the single-file CLI with small internal packages:
   - `internal/hookio`
   - `internal/store`
   - `internal/capsule`
3. Implement the six hook subcommands.
4. Add unit tests for event parsing, path resolution, manifest writing, and capsule output.
5. Update `docs/usage.md`, `docs/architecture.md`, and `docs/getting-started.md`.
6. Run `dev/agent/check-fast`.
7. Commit the slice.

## Later slices

- Hook installer: write Claude `.claude/settings.json` and Codex hook config snippets after review.
- Transcript parsing adapters for Claude JSONL and Codex transcript shapes.
- Decision and tool-result extraction.
- Size limits and chunking.
- Reference resolver command: `compactor open <ref>`.
- MCP server for resolving `compactor://` references.
- Config file for output root, max capsule size, raw capture policy, and redaction rules.
- Golden fixtures from sanitized hook events.

## Open questions

- Should default storage be repo-local `.compactor/` or user-data-root with a repo-local pointer?
- Should `inject` be wired primarily to `SessionStart(source=compact)` or `UserPromptSubmit` for each agent?
- Should raw transcript capture be opt-in forever, or enabled behind a local-only config?
- Do we want a custom URI scheme in v1, or plain file paths until the resolver exists?
