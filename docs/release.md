# Release

Release packaging is configured with GoReleaser. Tags publish GitHub release assets and update the Homebrew cask in `tomnagengast/homebrew-tap`.

## Prerequisites

- GoReleaser v2.
- GitHub `contents:write` permission through the workflow `GITHUB_TOKEN`.
- `HOMEBREW_TAP_GITHUB_TOKEN` with write access to `tomnagengast/homebrew-tap`.

## Local verification

```sh
dev/agent/check-full
goreleaser check
goreleaser release --snapshot --clean
```

`dev/agent/check-full` runs `goreleaser check` when GoReleaser is installed.

## Publish

1. Update `CHANGELOG.md`.
2. Confirm `compactor --version` reports the intended version metadata in a snapshot build.
3. Tag an annotated semver release:

   ```sh
   git tag -a v0.1.2 -m "v0.1.2"
   git push origin v0.1.2
   ```

4. The `release` workflow publishes GitHub release assets, checksums, and the Homebrew cask.

## Post-release smoke

```sh
brew tap tomnagengast/tap
brew install --cask tomnagengast/tap/compactor-cli
compactor --version
compactor hooks snippet claude --binary compactor
printf '{"session_id":"release-smoke","cwd":"%s","hook_event_name":"PreCompact","trigger":"manual"}\n' "$PWD" \
  | compactor hook claude precompact
```
