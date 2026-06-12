# Verifier

The MVP verifier stack has three levels:

- Static safety checks in Go.
- Optional user-supplied external UCI engine path.
- Optional user-supplied external tablebase probe.

External UCI verification uses `searchmoves` for each LLM candidate, compares centipawn loss against the best candidate, and rejects candidates that exceed the configured loss threshold. Verifier use is disclosed in every decision trace through the assistance block and verifier trace. Pure mode disables verifier scoring except deterministic legality.

External tablebase probing is configured with `verifier.tablebase_enabled`, `verifier.tablebase_path`, and `verifier.tablebase_timeout_ms`. The probe is a separate executable that reads JSON on stdin:

```json
{"fen":"8/8/8/8/8/8/8/K6k w - - 0 1","candidates":["h1h2"]}
```

It returns JSON on stdout:

```json
{"available":true,"best_moves":["h1h2"],"wdl":"win","dtz":1,"category":"win"}
```

When exact data is available, candidates outside `best_moves` are rejected and the decision trace includes `tablebase_*` details. Probe failures are recorded as verifier errors and degrade to the base verifier result.
