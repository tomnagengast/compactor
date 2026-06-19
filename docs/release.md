# Release

There is no packaged release yet.

Expected release path once the CLI has a useful workflow:

1. Run `dev/agent/check-full`.
2. Update `CHANGELOG.md`.
3. Confirm version metadata and CLI description.
4. Tag with `v*.*.*`.
5. Publish GitHub release assets and checksums.
6. Verify install from the release artifact.
7. Add and verify a Homebrew cask when distribution is ready.

Release automation should follow the sibling Go CLI pattern after the command surface settles.

