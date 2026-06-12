# Architecture

Noema64 is three products sharing one core:

```text
cmd/noema64        CLI
cmd/noema64-uci    protocol-safe UCI engine
cmd/noema64-gui    Wails v2 desktop app
```

Core packages:

- `chesscore`: deterministic chess wrapper. It does not import GUI, UCI, providers, strategy, or verifier code.
- `strategy`: memory schema, prompt construction, JSON parsing, and move repair.
- `providers`: LLM adapter interface with mock and OpenAI-compatible implementations.
- `verifier`: static safety and optional external UCI verifier boundary.
- `decision`: move decision state machine, scoring, fallback, and traces.
- `engine`: stateful game engine used by CLI, UCI, and app services.
- `uci`: protocol loop with stdout-only UCI lines.
- `appsvc`: Wails-facing service DTO boundary.
- `storage`: local YAML settings and JSONL traces.

Move pipeline:

```text
Snapshot position
Extract features
Build prompt
Call provider with deadline
Parse strict JSON
Normalize/repair moves
Verify candidates
Score candidates
Apply final legal move
Merge memory
Emit trace
```

The final move is rechecked against `chesscore` immediately before it mutates game state.

