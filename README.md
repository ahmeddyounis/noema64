# Noema64

Noema64 is an open-source explainable chess engine that uses a language model as a persistent strategic planner while deterministic Go code owns legal move validation, game state, fallback, UCI protocol behavior, traces, and local persistence.

It is not a Stockfish replacement. The MVP is optimized for legal full-game play, inspectable strategy memory, provider failure recovery, and protocol-safe UCI use.

## MVP Status

Implemented in this repository:

- Go chess core wrapper with legal moves, FEN, PGN, move history, outcomes, promotions, castling, and en passant through `github.com/corentings/chess/v2`.
- Strategy memory v1.2 structs, diffing, prompt builder, strict JSON parsing, candidate repair, and schema validation.
- Mock provider that works offline and an OpenAI-compatible HTTP adapter shape.
- Deterministic fallback ladder that always chooses a legal move when legal moves exist.
- Static blunderguard verifier plus optional external UCI verifier path support.
- UCI binary with `uci`, `isready`, `ucinewgame`, `position`, `go`, `stop`, `quit`, and `setoption`.
- Wails v2 GUI entrypoint with embedded board, time controls, recent games, settings, strategy, candidates, trace, resignation, PGN/FEN export, and benchmark controls.
- CLI and benchmark commands.
- Local YAML settings, JSONL decision traces, redacted game snapshots with strategy memory, tests, prompts, configs, and docs.

## Requirements

- Go 1.22 or newer.
- Node.js for frontend syntax and GUI smoke checks.
- Wails v2 CLI for packaged desktop builds.
- Optional: a local OpenAI-compatible LLM endpoint or cloud endpoint.
- Optional: a user-supplied UCI verifier such as Stockfish. No verifier binary is bundled.

## Quick Start

```sh
go test ./...
npm --prefix cmd/noema64-gui/frontend test
go run ./cmd/noema64 -cmd state
go run ./cmd/noema64 -cmd engine
go run ./cmd/noema64-bench -games 100
```

UCI smoke:

```sh
printf 'uci\nisready\nucinewgame\nposition startpos moves e2e4 e7e5 g1f3\ngo movetime 100\nquit\n' | go run ./cmd/noema64-uci
```

GUI development:

```sh
cd cmd/noema64-gui
wails dev
```

The embedded frontend is static and works with Wails bindings. The mock provider is the default, so no API key is required. The GUI restores the most recent saved game, including clock and strategy state, from the configured log directory on relaunch.

## Modes

- `pure`: provider candidates are repaired and legality-filtered; no tactical verifier is used.
- `blunderguard`: default mode; static verifier can reject obvious mate-in-one risks and discloses assistance.
- `hybrid`: scoring reserves more weight for verifier/search-style signals.
- `coach`: uses the same legal pipeline with teaching-oriented personality settings.

## Provider Setup

Default config uses the offline mock provider. To use an OpenAI-compatible endpoint, set:

```yaml
llm:
  provider: openai_compatible
  endpoint: http://localhost:11434/v1
  model: your-model
  api_key: ""
```

Cloud provider mode may send FEN, legal moves, move history, strategy memory, and settings to the configured provider. Raw prompt logging is off by default.

## Verifier Setup

Static verification is built in. External UCI verification is optional:

```yaml
verifier:
  enabled: true
  kind: uci
  path: /usr/local/bin/stockfish
  movetime_ms: 100
  max_centipawn_loss: 180
```

Noema64 does not bundle Stockfish in the MVP. When configured, the external UCI verifier analyzes LLM candidate moves with `searchmoves`, compares centipawn loss against the best candidate, and records verifier decisions in the trace.

## UCI Example

```text
uci
isready
ucinewgame
position startpos moves e2e4 e7e5 g1f3
go wtime 300000 btime 300000 winc 2000 binc 2000
quit
```

The UCI process writes protocol output only to stdout. JSON traces are written to local files when enabled. GUI game snapshots are stored under `logging.output_dir/games`.

## Limitations

- The static verifier is intentionally shallow.
- The GUI is a Wails MVP surface, not a polished app-store release.
- LLM quality depends on the configured provider.
- Raw prompts are not stored unless the user enables logging.

## License

Noema64 source is MIT licensed. Optional external verifier binaries retain their own licenses.
