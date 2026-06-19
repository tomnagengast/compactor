# Architecture

`compactor` is intended to sit beside an agent rather than replace the agent runtime. Its job is to receive context that is about to leave the active window, preserve that context as local documents, and hand back a compact reference layer.

## Model

The early model has four parts:

- Source context: message history, tool outputs, local notes, and explicit compaction summaries supplied by the agent or harness.
- Document store: local markdown or structured files written under a predictable project or session path.
- Reference index: a small agent-readable map containing stable ids, short summaries, paths, and retrieval hints.
- Adapter layer: Claude and Codex-specific integration surfaces, prompts, commands, hooks, or documented manual flows.

## Constraints

The first implementation should be local-first, deterministic where possible, and careful with sensitive data. It should avoid hosted storage and private APIs unless a later product decision changes that boundary.

Generated documents should be useful to agents after context loss. That means stable paths, concise summaries, clear provenance, and enough metadata for an agent to decide whether opening a document is worth the context cost.

## Open design questions

- Where can Claude and Codex reliably intercept or influence compaction?
- Is the first workflow automatic, manual, or a documented prompt pattern?
- Should documents live inside the repo, under an agent data directory, or in a project-local hidden directory?
- What metadata is enough for retrieval without rebuilding a full search engine?
- How should references handle private tool output, secrets, and user-redacted material?

