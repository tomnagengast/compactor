# Compaction survey

Date: 2026-06-19

This note summarizes the official Claude and Codex compaction surfaces that matter for `compactor`.

## Claude

Claude Code treats the context window as the working memory for a session. Startup context can include `CLAUDE.md`, auto memory, MCP tool names, and skill descriptions. During a long session, file reads, rules, hook output, and conversation history add to that window. The Claude Code context-window docs describe `/compact` as replacing the conversation with a structured summary; automatic compaction uses the same mechanism when the window fills.

Compaction survival is uneven. Root `CLAUDE.md` and unscoped rules are reloaded from disk. Auto memory is reloaded. Path-scoped rules and nested `CLAUDE.md` files are lost until a matching file is read again. Invoked skill bodies are re-injected after compaction, but capped and truncated. This matters because `compactor` should prefer durable files plus a small index over relying on transient message history.

Claude Code exposes compaction lifecycle hooks. `PreCompact` receives the session id, transcript path, cwd, trigger, and custom instructions. It can block compaction. `PostCompact` receives the same basic context plus `compact_summary`, but cannot change the compaction result. This makes Claude the stronger first target for an automatic adapter: the hook payload has both the transcript path and the generated compact summary.

The Claude Agent SDK can send `/compact` as a prompt command and observes a `compact_boundary` system event with pre-compaction token metadata. The Claude API also has beta server-side compaction through `context_management.edits`, producing a `compaction` block that must be passed back on later turns. API compaction supports trigger thresholds, custom instructions, and `pause_after_compaction`, which is especially relevant for preserving recent messages or adding references after a summary is produced.

Sources:

- [Claude Code context window](https://code.claude.com/docs/en/context-window)
- [Claude Code hooks reference](https://code.claude.com/docs/en/hooks)
- [Claude Code Agent SDK slash commands](https://code.claude.com/docs/en/agent-sdk/slash-commands)
- [Claude API compaction](https://platform.claude.com/docs/en/build-with-claude/compaction)
- [Claude API context windows](https://platform.claude.com/docs/en/build-with-claude/context-windows)

## Codex

Codex documents compaction as a way to keep long threads within the model context window. It can compact automatically and also exposes `/compact` in the CLI to summarize the visible conversation. `/status` shows context usage, and `codex resume` reopens saved local sessions from transcripts under `$CODEX_HOME`.

Codex config includes `model_auto_compact_token_limit`, `compact_prompt`, and `experimental_compact_prompt_file`. That gives `compactor` a direct path for influencing what gets summarized, even before deeper integration exists.

Codex hooks include `PreCompact` and `PostCompact`, with a `trigger` value of `manual` or `auto`. `PreCompact` can stop compaction by returning `continue: false`; `PostCompact` runs after compaction. The documented Codex hook payload is thinner than Claude's current payload for this use case because it identifies the turn and trigger but does not document a compact summary field.

Codex also exposes `UserPromptSubmit`, where additional context can be injected before a prompt is processed. That may be useful for resolving `compactor://` references or reminding the agent where the generated document index lives.

Sources:

- [Codex CLI slash commands](https://developers.openai.com/codex/cli/slash-commands)
- [Codex CLI features](https://developers.openai.com/codex/cli/features)
- [Codex configuration reference](https://developers.openai.com/codex/config-reference)
- [Codex hooks](https://developers.openai.com/codex/hooks)

## Initial design implications

The core product should separate capture, document writing, reference emission, and reinjection. Claude and Codex can then share a document model while keeping agent-specific adapters thin.

The first adapter should probably be hook-driven for Claude and prompt/config-driven for Codex. Claude's `PostCompact` hook can archive the compact summary and transcript-derived documents after the native compaction has run. Codex's `compact_prompt` or `experimental_compact_prompt_file` can ask Codex to emit document references during compaction, while `PreCompact`/`PostCompact` hooks can snapshot local state around the event.

The document store should be local, project-scoped by default, and easy for agents to browse. A likely shape is `.compactor/sessions/<agent>/<session-id>/` with:

- `index.md`: agent-readable map of compacted documents.
- `timeline.md`: chronological compacted message history.
- `decisions.md`: durable decisions and constraints.
- `tool-results/`: large command outputs or logs split into files.
- `manifest.json`: stable ids, source paths, timestamps, token estimates, and privacy flags.

The reference layer should be tiny enough to survive future compactions. References should include path, stable id, one-line summary, and suggested retrieval condition. Example: `compactor://session/<id>/decisions#d004` plus a local file path fallback.

Open questions:

- Should `compactor` write inside the repo by default, or under the agent's data directory with a repo-local pointer?
- Should the first workflow read raw transcript JSONL, hook payloads, or explicit stdin from a compaction prompt?
- Should references be plain markdown links first, with a custom URI scheme later?
- How much should `compactor` try to classify content versus simply preserving and indexing it?
- How do we redact or skip sensitive tool outputs without losing provenance?

