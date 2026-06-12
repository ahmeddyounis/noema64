# Contributing

Noema64 is split into independent seams:

- `internal/chesscore`: deterministic chess state and notation.
- `internal/strategy`: prompts, memory, schema parsing, repair.
- `internal/providers`: mock, local, and cloud LLM adapters.
- `internal/verifier`: static checks and optional UCI verifier support.
- `internal/decision`: move pipeline, scoring, fallback, traces.
- `internal/uci`: UCI protocol loop.
- `internal/appsvc`: Wails-facing services and DTOs.

Before opening a PR:

```sh
gofmt -w .
go test ./...
go run ./cmd/noema64-bench -games 100
```

Behavior changes should name the relevant requirement IDs from the PRD, for example `MOVE-002` or `UCI-001`.

