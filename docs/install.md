# Install

`compactor` is not packaged yet. Build from source for now:

```sh
git clone https://github.com/tomnagengast/compactor
cd compactor
go build ./cmd/compactor
./compactor --help
```

For development checks:

```sh
dev/agent/check-fast
```

Future release work should add versioned GitHub release assets, checksums, and a Homebrew cask following the sibling CLI pattern.

