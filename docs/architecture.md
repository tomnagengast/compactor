# Architecture

`compactor` is intended to sit beside an agent rather than replace the agent runtime. Its job is to receive context that is about to leave the active window, preserve that context as local documents, and hand back a compact reference layer.

## Model

The model has four parts:

- Source context: message history, tool outputs, local notes, and explicit compaction summaries supplied by the agent or harness.
- Document store: local markdown or structured files written under a predictable project or session path.
- Reference index: a small agent-readable map containing stable ids, short summaries, paths, and retrieval hints.
- Adapter layer: Claude and Codex-specific integration surfaces, prompts, commands, hooks, or documented manual flows.

The first implementation uses hook commands:

- `precompact`: snapshot hook metadata and write session documents before native compaction.
- `postcompact`: capture native compact metadata after compaction and refresh the pending context capsule.
- `inject`: emit the small progressive-disclosure capsule through `additionalContext`.

## Constraints

The first implementation should be local-first, deterministic where possible, and careful with sensitive data. It should avoid hosted storage and private APIs unless a later product decision changes that boundary.

Generated documents should be useful to agents after context loss. That means stable paths, concise summaries, clear provenance, and enough metadata for an agent to decide whether opening a document is worth the context cost.

Prompt caching is an explicit constraint. Hooks should avoid mutating root instructions, hook config, tool definitions, MCP config, or other early prompt layers during a session. Dynamic detail belongs in `.compactor/` files. Injected context should stay small, deterministic, and reference-oriented.

Transcript parsing is bounded and local. `timeline.md`, `decisions.md`, and `tool-results.md` may include short extracted snippets plus promoted Claude/Codex metadata such as stable item IDs, parent IDs, tool names, and compaction boundary markers. Full raw transcripts are not copied into `.compactor/` by default. Decision extraction is heuristic and should be treated as a candidate list until the agent checks source context.

## Open design questions

- Where can Claude and Codex reliably intercept or influence compaction?
- Is the first workflow automatic, manual, or a documented prompt pattern?
- Should documents live inside the repo, under an agent data directory, or in a project-local hidden directory?
- What metadata is enough for retrieval without rebuilding a full search engine?
- How should references handle private tool output, secrets, and user-redacted material?
- Should `inject` be wired primarily to `SessionStart(source=compact)` or `UserPromptSubmit` for each agent?
- Which larger tool outputs should be split into separate referenced documents instead of remaining bounded inline extracts?

See [research/compaction-survey.md](./research/compaction-survey.md) for the current official-docs summary of Claude and Codex compaction surfaces.
