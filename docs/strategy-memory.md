# Strategy Memory

Strategy memory is structured public state, not raw private reasoning. It stores:

- Current plan summary, status, confidence, and horizon.
- Targets.
- Piece improvement goals.
- Pawn breaks.
- Opponent model.
- Commitments.
- Refutation triggers.
- Tactical warnings.
- Last update metadata.

Every engine move produces before/after memory and a compact diff in the decision trace.

Public schema artifacts live under `schemas/`:

- `schemas/strategy_memory.schema.json`
- `schemas/move_decision.schema.json`
