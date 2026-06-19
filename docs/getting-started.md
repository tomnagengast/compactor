# Getting started

The first useful workflow is currently repo verification:

```sh
go run ./cmd/compactor --help
dev/agent/check-fast
```

The planned first product workflow is:

1. Capture or receive the context that would otherwise be compacted.
2. Split the history into durable documents with stable identifiers.
3. Write an agent-readable index that summarizes each document.
4. Return compact references that can replace the original bulk context.
5. Let the agent reopen only the documents needed for the next task.

Keep this page current as soon as the first real command exists.

