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
- `ponderhit`
- `stop`
- `quit`
- `setoption name <option> value <value>`

UCI stdout is protocol-only. Diagnostics use `info string`; JSONL traces go to local files. Search completion emits Noema64 `info string` lines for mode/assistance/fallback, provider/model/prompt metadata, selected move, verifier status, timing, and legal-move count.

Additional Noema64 options:

- `Mode`
- `Personality`
- `LLMProvider`
- `LLMEndpoint`
- `LLMModel`
- `Temperature`
- `MaxCandidates`
- `LLMRetries`
- `MoveOverhead`
- `MaxProviderMillis`
- `VerifierEnabled`
- `VerifierPath`
- `VerifierMoveTime`
- `MaxVerifierMillis`
- `VerifierMaxCentipawnLoss`
- `TablebaseEnabled`
- `TablebasePath`
- `TablebaseTimeoutMS`
- `TraceEnabled`
- `TraceFile`
- `LogPath`

`Temperature` is exposed as a UCI spin value from `0` to `200`, mapped to runtime temperatures `0.0` through `2.0`. `MoveOverhead` subtracts milliseconds from `go` time-control budgets before starting a search. `MaxProviderMillis` caps the engine move timeout, and `MaxVerifierMillis` is accepted as a wider-range alias for external verifier thinking time. `TraceFile` points at a specific JSONL trace file; `LogPath` is accepted as a compatibility alias for the same setting. If neither is set, Noema64 writes decision traces under the configured local log directory.

`ponderhit` is accepted while a search is active and acknowledged with `info string ponderhit accepted`. Noema64 does not start a separate speculative ponder search yet; inactive `ponderhit` is ignored with a protocol-safe `info string`.

For Lichess bot bridge usage, see [lichess-bot.md](lichess-bot.md). Noema64 stays a local UCI engine; it does not implement or bundle a Lichess API client.
