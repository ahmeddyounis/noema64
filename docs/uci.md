# UCI

Run:

```sh
go run ./cmd/noema64-uci
```

Supported MVP commands:

- `uci`
- `isready`
- `ucinewgame`
- `position startpos`
- `position startpos moves ...`
- `position fen <fen>`
- `position fen <fen> moves ...`
- `go depth <n>`
- `go movetime <ms>`
- `go wtime <ms> btime <ms> winc <ms> binc <ms>`
- `stop`
- `quit`
- `setoption name <option> value <value>`

UCI stdout is protocol-only. Diagnostics use `info string`; JSONL traces go to local files.

Additional Noema64 options:

- `Mode`
- `Personality`
- `LLMProvider`
- `LLMEndpoint`
- `LLMModel`
- `Temperature`
- `MaxCandidates`
- `VerifierEnabled`
- `VerifierPath`
- `VerifierMoveTime`
- `VerifierMaxCentipawnLoss`
- `TraceEnabled`
- `TraceFile`

`Temperature` is exposed as a UCI spin value from `0` to `200`, mapped to runtime temperatures `0.0` through `2.0`. `TraceFile` points at a specific JSONL trace file; if unset, Noema64 writes decision traces under the configured local log directory.
