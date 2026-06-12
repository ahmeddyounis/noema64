# Verifier

The MVP verifier stack has two levels:

- Static safety checks in Go.
- Optional user-supplied external UCI engine path.

External UCI verification uses `searchmoves` for each LLM candidate, compares centipawn loss against the best candidate, and rejects candidates that exceed the configured loss threshold. Verifier use is disclosed in every decision trace through the assistance block and verifier trace. Pure mode disables verifier scoring except deterministic legality.
