# Noema64

Noema64 is an open-source explainable chess engine that uses a language model as a persistent strategic planner while deterministic Go code owns legal move validation, game state, fallback, UCI protocol behavior, traces, and local persistence.

It is not a Stockfish replacement. The project is optimized for legal full-game play, inspectable strategy memory, provider failure recovery, study workflows, and protocol-safe UCI use.

## MVP Status

Implemented in this repository:

- Go chess core wrapper with legal moves, FEN, PGN, move history, outcomes, promotions, castling, and en passant through `github.com/corentings/chess/v2`.
- Strategy memory v1.2 structs, diffing, versioned editable prompt packs, strict JSON parsing, candidate repair, and schema validation.
- Mock provider that works offline, OpenAI, OpenAI-compatible HTTP, Anthropic, Gemini, Ollama, and local policy-prior provider adapters.
- Deterministic fallback ladder that always chooses a legal move when legal moves exist.
- Static blunderguard verifier plus optional external UCI verifier and external tablebase probe path support.
- UCI binary with `uci`, `isready`, `ucinewgame`, `position`, `go`, `stop`, `quit`, and `setoption`.
- Wails v2 GUI entrypoint with embedded board, time controls, recent games, settings, strategy, candidates, trace, resignation, PGN/FEN/JSONL trace export, and benchmark controls.
- Study dashboard, compressed memory, plan coherence, candidate diversity, deterministic multi-agent review, and editable strategy memory APIs.
- Chess960/custom-start metadata, deterministic Chess960 start generation, and Noema64-managed Chess960 castling through a compatibility layer.
- Local backup/restore archives, fine-tune JSONL dataset export, deterministic tournament/rating automation, and sandbox validation for configured external binaries.
- CLI and benchmark commands with JSON or CSV benchmark output.
- Local YAML settings, JSONL decision traces, redacted game snapshots with strategy memory, tests, prompts, configs, and docs.

## Requirements

- Go 1.22 or newer.
- Node.js for frontend syntax and GUI smoke checks.
- Wails v2 CLI for packaged desktop builds.
- Optional: an OpenAI API key, local OpenAI-compatible LLM endpoint, or other cloud provider key.
- Optional: a user-supplied UCI verifier such as Stockfish or an external tablebase probe. No verifier or tablebase binary is bundled.

## Quick Start

```sh
go test ./...
npm --prefix cmd/noema64-gui/frontend test
go run ./cmd/noema64 -cmd state
go run ./cmd/noema64 -cmd engine
go run ./cmd/noema64 -cmd analyze -fen 'r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3'
go run ./cmd/noema64 -cmd state -variant chess960 -seed 42
go run ./cmd/noema64 -cmd study
go run ./cmd/noema64 -cmd tournament -games-per-pair 1
go run ./cmd/noema64-bench -games 100
go run ./cmd/noema64-bench -games 100 -format csv > benchmark.csv
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

CI runs Go format/vet/tests, race tests for the non-Wails packages, frontend smoke tests, UCI smoke, trace validation, perft, dependency license scanning, non-GUI binary builds, and the 100-game reliability benchmark. Packaged Wails desktop builds are a local/manual release gate because hosted Linux runners require platform GUI system dependencies.

## Modes

- `pure`: provider candidates are repaired and legality-filtered; no tactical verifier is used.
- `current`: "Best now" mode; resets strategy memory for each decision and ranks current-position tactics/search over long-term plan continuity.
- `blunderguard`: default mode; static verifier can reject obvious mate-in-one risks and discloses assistance.
- `hybrid`: scoring reserves more weight for verifier/search-style signals.
- `coach`: uses the same legal pipeline with teaching-oriented personality settings.

Personality profiles are not display-only. Their risk tolerance is included in the arbiter as a small `personality_score` on each candidate, so aggressive profiles can break close ties toward safe forcing moves while positional and coach profiles lean toward clearer low-risk choices.

Current and hybrid modes record `deterministic_mcts_material` search assistance in the decision trace. It is a bounded deterministic playout scorer, not a claim of engine-strength MCTS.

## Study And Roadmap Workflows

The GUI toolbar includes Study and Lab dialogs. The same workflows are available from the CLI:

```sh
go run ./cmd/noema64 -cmd study
go run ./cmd/noema64 -cmd agents
go run ./cmd/noema64 -cmd book
go run ./cmd/noema64 -cmd compare
go run ./cmd/noema64 -cmd backup -backup-dir runs/backups
go run ./cmd/noema64 -cmd finetune
go run ./cmd/noema64 -cmd tournament -games-per-pair 2
```

Study returns compressed memory, opening-book suggestions, endgame trainer drills, plan coherence, candidate diversity, lesson/puzzle prompts, heatmap data, and deterministic strategist/critic/tactician/arbiter reviews. Lab workflows create local zip backups, restore backups into a target directory, compare pure vs hybrid analysis, compare prompt packs, build custom personality profile drafts, export local fine-tune JSONL examples from sanitized traces, and run engine-vs-engine tournament ratings across core modes.

## Provider Setup

Default config uses the offline `mock` provider, so Noema64 can run without an API key.

From the GUI, open Settings, choose a provider profile or provider, enter a model, then save. For `OpenAI`, the endpoint field is managed automatically and uses `https://api.openai.com/v1`; you only need the model and API key. For `OpenAI-compatible`, enter the endpoint for your local or remote compatible server.

Supported provider values:

| Provider | Use case | Endpoint |
| --- | --- | --- |
| `mock` | Offline demos, tests, and CI | Not used |
| `openai` | OpenAI API | Managed automatically |
| `openai_compatible` | Local or hosted OpenAI-compatible chat completions API | Required |
| `anthropic` | Anthropic Messages API | Configured endpoint |
| `gemini` | Gemini generateContent API | Configured endpoint |
| `ollama` | Local Ollama JSON chat | Configured endpoint |
| `policy_prior` | Local exact-position policy-prior model | Model path |

For OpenAI in YAML, set the provider and model:

```yaml
llm:
  provider: openai
  model: your-openai-model
  api_key: ""
```

To use another OpenAI-compatible endpoint in YAML, set:

```yaml
llm:
  provider: openai_compatible
  endpoint: http://localhost:11434/v1
  model: your-model
  api_key: ""
```

Provider settings support `api_key` or `api_key_ref`. In the GUI, the keychain action stores a typed key in the OS keychain when supported and replaces the raw key with a reference.

Endpoint-backed provider modes may send FEN, legal moves, move history, strategy memory, and settings to the configured provider, including OpenAI, local or remote OpenAI-compatible endpoints, and Ollama endpoints. The GUI requires an explicit data-sharing acknowledgement before saving those providers. Raw prompt logging is off by default.

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

External verifier and tablebase paths are executed without a shell and are validated before launch. Simple PATH binary names such as `stockfish` are allowed; paths containing whitespace, control characters, path traversal, or non-executable absolute targets are rejected.

External tablebase probing is also optional. Configure a probe executable that reads `{ "fen": "...", "candidates": ["e2e4"] }` JSON from stdin and returns exact tablebase JSON on stdout:

```yaml
verifier:
  tablebase_enabled: true
  tablebase_path: /usr/local/bin/noema64-tablebase
  tablebase_timeout_ms: 1000
```

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

For bot bridge usage, see [docs/lichess-bot.md](docs/lichess-bot.md).

## Limitations

- The static verifier is intentionally shallow.
- Chess960 castling is handled by Noema64's compatibility layer rather than the upstream move generator.
- The GUI is a Wails MVP surface, not a polished app-store release.
- LLM quality depends on the configured provider.
- Raw prompts are not stored unless the user enables logging.

## License

Noema64 source is MIT licensed. Optional external verifier binaries retain their own licenses.
