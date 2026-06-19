# Post-core hardening implementation plan

> updated: 2026-06-19T16:02:39-07:00

## Goal

Harden `compactor` after the core hook loop. Make installs explain themselves, improve transcript-derived documents, validate against real-shaped data, dogfood the hook flow safely, then package a first release.

Keep the project local-first, deterministic, prompt-cache-aware, and privacy-conservative. Do not copy raw transcripts by default. Keep `pending-context.md` tiny and reference-oriented.

## Sequence

1. Hook install diagnostics.
2. Sanitized fixture hardening.
3. Deterministic extraction quality.
4. Large tool output split-outs.
5. Dogfood workflow.
6. Release packaging.

## Slice 1: Hook install diagnostics

Commit: `add hook install diagnostics`

Current gap: `hooks install` and `hooks uninstall` print target/mode/final JSON, but not what changed. Malformed-but-decodable hook shapes are also quiet.

Implementation:

- Extend `internal/install.Plan` with `Diagnostics []Diagnostic`.
- Add `Diagnostic{Level, Action, Event, Message}` or similarly small fields.
- Make `merge` and `remove` return diagnostics.
- Report event-level actions only: `add`, `skip`, `remove`, `missing`, `preserve`, `warn`.
- Warn when `hooks` is not an object or an event value is not an array.
- Add diagnostics between `mode` and JSON in `DryRun()`.
- Keep write output terse; optionally add counts only.

Tests:

- New-target dry run reports added hooks.
- Repeated install reports skipped existing hooks.
- Malformed `hooks` shape warns.
- Malformed event shape warns.
- Uninstall reports removed and missing generated hooks.
- CLI smoke checks `diagnostics:` appears.

Docs:

- Update `docs/usage.md` install section with a tiny diagnostics example.
- Remove diagnostics from README roadmap.

## Slice 2: Sanitized fixture hardening

Commit: `add sanitized transcript fixtures`

Current gap: parser and store tests use useful synthetic inline JSONL, but not real-shaped fixtures. We need schema realism without private content.

Implementation:

- Add committed fixtures under Go `testdata`, not `.compactor/`.
- Start with:
  - `internal/transcript/testdata/claude-real-sanitized.jsonl`
  - `internal/transcript/testdata/codex-real-sanitized.jsonl`
  - optional `internal/store/testdata/claude-hook-flow/`
- Keep fixtures small, 8-20 lines, preserving real structural keys.
- Sanitize raw transcripts outside the repo only, then manually review before copying into `testdata`.
- Add leak-test helper that scans fixtures for risky patterns.
- Prefer expected metadata assertions over full markdown goldens.

Scrub policy:

- Replace names, emails, phones, home paths, private URLs, hostnames, tokens, env values, file content, and long command output.
- Preserve structural fields: `message.content`, `toolUseResult`, `item`, `payload`, `event_msg`, `uuid`, `parentUuid`, `call_id`, `type`, `name`, compact boundary fields.
- Use placeholders such as `USER_ALPHA`, `/Users/example/project`, `TOKEN_REDACTED`, `[TOOL_OUTPUT_REDACTED: 18342 bytes]`.

Tests:

- Claude fixture promotes UUID, parent UUID, tool names, tool-result kind, decisions.
- Codex fixture promotes item IDs, call IDs, function/tool names, compaction boundary.
- Store/CLI tests assert privacy flags, bounded extracts, no raw transcript copy, pending capsule size, and `validate` success.
- Leak test fails on private names/domains, `/Users/tom`, likely keys, emails, private key headers, long base64-like strings, and oversized lines without redaction markers.

Docs:

- Add sanitized fixture policy to `docs/contributing.md` or `docs/agents/README.md`.

## Slice 3: Deterministic extraction quality

Commit: `add deterministic extraction rules`

Current gap: decisions and tool results are flat `Finding{Line, Kind, Text}` lists produced by simple keyword scans.

Implementation:

- Keep `internal/transcript` focused on JSONL parsing and agent normalization.
- Add `internal/extract` for deterministic classification over `[]transcript.Entry`.
- Introduce richer finding metadata:
  - `ID`
  - `Category`
  - `Kind`
  - `Confidence`
  - `Line`
  - `EntryID`
  - `ParentID`
  - `Role`
  - `ToolName`
  - `ToolCallID`
  - `Text`
  - `Reason`
- Categories: `decision`, `constraint`, `open-question`, `next-action`, `tool-call`, `tool-result`, `tool-failure`, `compaction-boundary`.
- Split message text into bounded clauses before classification.
- Deduplicate by normalized text plus category.
- Generate stable IDs from category, line, and sequence.

Rules:

- High-confidence decisions: `Decision:`, `Decided:`, `We decided`, `Chosen approach`, `Use X for Y`, `Keep X as Y`.
- Constraints: `must`, `must not`, `never`, `do not`, `only`, `avoid`, `local-first`, `deterministic`, `bounded`, `non-goal`.
- Open questions: explicit labels and architecture/product question sentences.
- Next actions: `next step`, `next slice`, `follow up`, `todo`.
- Downgrade weak markers like `let's` and `we should`.
- Suppress false positives from stack traces, test output, quoted logs, and tool result text unless explicitly tagged.

Output:

- `decisions.md` sections: `Decisions`, `Constraints`, `Open questions`, `Next actions`, `Low-confidence candidates`.
- `tool-results.md` sections: `Tool calls`, `Tool results`, `Failures or warnings`.
- Include line, optional entry ID, confidence, and reason.
- Do not increase `pending-context.md` detail.

Tests:

- Explicit decisions, constraints, open questions, next actions.
- False-positive fixture.
- Claude and Codex tool call/result fixture.
- Large-output fixture proves no raw log copying.
- Determinism test runs same fixture twice and compares IDs/order.

## Slice 4: Large tool output split-outs

Commits:

- `extract large tool outputs`
- `write tool output documents`
- `document tool output storage`

Current gap: tool output references are bounded inline. Large outputs should become referenced local documents, without copying full raw transcripts.

Implementation:

- Add `transcript.ToolOutput` and `Snapshot.LargeToolOutputs`.
- Add `Options.LargeToolOutputBytes`; default around 4096 or 8192.
- Extract candidate raw output before `trimText` and before whitespace compaction.
- Detect only tool-result payloads, not long assistant/user prose.
- Trigger on Claude `tool_result`, `toolUseResult.stdout`, `stderr`, `is_error`; Codex tool/function/local shell results; generic `result`, `output`, `stdout`, `stderr` only when tool markers are present.
- Let `store` own file layout and manifest entries.

Layout:

```text
.compactor/sessions/<agent>/<session-id>/tool-results/
0001-<tool-or-kind>.md
0002-<tool-or-kind>.md
```

If needed, split very large outputs into deterministic parts:

```text
0001-shell-part-001.md
0001-shell-part-002.md
```

Manifest:

- Extend `Document` with optional metadata: `source_line`, `source_kind`, `tool_name`, `bytes`, `sha256`, `parent_id`, `excerpt`, `storage_policy`.
- Add each split output/chunk as a manifest document so `resolve` and `validate` work without special cases.
- Extend privacy block with `LargeToolOutputsStored`.

Privacy:

- Keep `RawTranscriptStored=false`.
- Store only scoped tool output payloads.
- Use `0600` permissions.
- Add simple sensitive-pattern guardrails before writing; skip or redact suspicious output and note omission in `tool-results.md`.

Output:

- Keep `tool-results.md` small: line, tool, bytes, hash prefix, excerpt, local path, `compactor://` ref.
- `pending-context.md` may mention count only, not every chunk.

Tests:

- Large Claude result creates a file.
- Large Codex/shell result creates a file.
- Long non-tool prose is not stored.
- Manifest includes split docs.
- `resolve` and `validate` cover split docs.
- Pending context excludes full output.

## Slice 5: Dogfood workflow

Commit: `document dogfood workflow`

Current gap: we have commands and tests, but no safe real-run protocol.

Implementation:

- Add `docs/dogfood.md`.
- Document manual hook simulation first.
- Document project-scope install previews before any `--write`.
- Use a sacrificial repo/workspace first.
- Build a dedicated binary outside hook config experiments:

```sh
go build -o /tmp/compactor-dogfood ./cmd/compactor
```

Scenarios:

- Manual fake Claude/Codex hook payloads with transcript paths.
- Claude `/compact`, expecting possible `compact_summary`.
- Codex `/compact`, watching for actual payload fields.
- Injection through `SessionStart` and `UserPromptSubmit`.

Validation checklist:

- `compactor validate <session-dir>` passes.
- `compactor resolve <ref>` works for every manifest doc.
- `pending-context.md` contains refs/paths, not full summaries.
- `timeline.md` records hook phase/event log/transcript metadata.
- `decisions.md` and `tool-results.md` are useful enough to recover context.
- Hook output stays small and continues native compaction on errors.

Observations to capture:

- Actual hook payload fields.
- Whether `transcript_path` is consistently present.
- Whether Codex provides compact summary data.
- Whether `UserPromptSubmit` injection is too noisy.
- Prompt-cache churn or startup warnings.
- Hook config incompatibilities.

## Slice 6: Release packaging

Commits:

- `add release version metadata`
- `add goreleaser packaging`
- `add release workflow`
- `document packaged installs`
- `prepare v0.1.0 changelog`

Current gap: docs say no packaged release. `--version` works through `main.version = "dev"`, but release stamping and assets do not exist.

Implementation:

- Use GoReleaser.
- Keep pure-Go release simple; no signing/notarization in v0.1 unless Gatekeeper feedback forces it.
- Add version metadata, either minimal `-X main.version={{ .Version }}` or `internal/version`.
- Add `.goreleaser.yaml`:
  - `project_name: compactor`
  - `main: ./cmd/compactor`
  - binary `compactor`
  - `CGO_ENABLED=0`
  - targets `darwin/linux`, `amd64/arm64`
  - `tar.gz` archives
  - checksums
  - snapshot version template
- Add tag-triggered `.github/workflows/release.yml`.
- Extend CI or `check-full` with `goreleaser check` and snapshot release when available.
- Configure Homebrew cask in `tomnagengast/homebrew-tap`, likely `compactor-cli`.

Docs:

- Update README install section.
- Update `docs/install.md`.
- Update `docs/release.md`.
- Update `CHANGELOG.md` for `v0.1.0`.

Release flow:

```sh
dev/agent/check-full
goreleaser check
goreleaser release --snapshot --clean
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Post-release smoke:

- Verify GitHub assets/checksums.
- Install via Homebrew.
- Run `compactor --version`.
- Run `compactor hooks snippet claude --binary compactor`.
- Run one hook smoke test.

## Verification

Every implementation slice should run:

```sh
dev/agent/check-fast
```

Fixture and release slices should also run:

```sh
dev/agent/check-full
```

Run leak scans before committing fixtures:

```sh
rg -n "PRIVATE|TOKEN|SECRET|/Users/tom|bajka|skyview|nagengast" internal/**/testdata docs
```

## Non-goals

- No default LLM-assisted extraction.
- No hosted services.
- No embeddings or vector search.
- No raw transcript capture by default.
- No broad redaction framework before simple fixture/output leak checks.
- No full hook config schema rewrite for diagnostics.
- No release signing/notarization in the first packaging slice.

## Unresolved questions

- Should fixture work include a reusable sanitizer command now, or start with manual scrub plus leak-test gate?
- Should large tool outputs be stored by default, or gated behind a config flag because tool logs are often sensitive?
- What exact large-output threshold should be the default: 4096, 8192, or larger?
- Should `UserPromptSubmit` injection remain default after dogfood, or should injection prefer only compact session start?
- Should release versioning stay as `main.version`, or move to an `internal/version` package before v0.1?
