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
compactor hooks install claude
compactor hooks install codex
compactor hooks uninstall claude
compactor hooks uninstall codex
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
- `timeline.md`: compaction event metadata plus bounded transcript extracts.
- `decisions.md`: bounded heuristic candidates for decisions, constraints, and open questions.
- `tool-results.md`: bounded references to tool calls and tool results.
- `summaries/native.md`: native compact summary when the hook payload provides one.
- `pending-context.md`: small reinjection capsule.

Transcript parsing is intentionally bounded. `compactor` does not copy full raw transcripts by default; it extracts short timeline entries, heuristic decision candidates, and tool-result references so agents know where to look next.

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

## Hook install

Use `hooks install` to merge the generated hook config into a target JSON file. The default is a dry run:

```sh
compactor hooks install claude --binary /absolute/path/to/compactor
compactor hooks install codex --binary /absolute/path/to/compactor
```

Project-scope targets:

- Claude: `.claude/settings.json`
- Codex: `.codex/hooks.json`

User-scope targets:

- Claude: `~/.claude/settings.json`
- Codex: `~/.codex/hooks.json`

Options:

- `--scope project|user`: choose the target layer. Defaults to `project`.
- `--binary <path>`: command path to put in hook config.
- `--write`: write the merged JSON. Without this flag, the command prints the target and resulting JSON.
- `--dry-run`: explicit no-write mode.

The installer preserves existing hook events and appends missing `compactor` hook groups. It does not edit Codex `config.toml`; Codex can load `hooks.json`, and using one hook representation per layer avoids startup warnings.

## Hook uninstall

Use `hooks uninstall` to remove the generated `compactor` hook groups while preserving unrelated hooks:

```sh
compactor hooks uninstall claude --binary /absolute/path/to/compactor
compactor hooks uninstall codex --binary /absolute/path/to/compactor
```

Like install, uninstall is a dry run unless `--write` is present. It removes hook groups by exact generated command string, so pass the same `--binary` value used during install.

Planned command areas:

- Add richer hook merge diagnostics.
- Improve transcript parsing with agent-specific adapters.
- Resolve a reference back to its source document.
- Validate generated documents for drift, missing references, and unsafe paths.
