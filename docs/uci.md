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

