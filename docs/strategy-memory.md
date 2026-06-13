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

Engine state also exposes computed strategy metrics:

- `quality`: combined completeness, consistency, and drift score.
- `completeness`: whether core memory fields such as plan, phase, targets, opponent model, commitments, triggers, and last update are populated.
- `consistency`: schema/status/side/confidence/horizon sanity checks.
- `drift`: how sharply the current memory changed from the previous decision memory.
- `alerts`: user-facing strategy quality and drift alerts for the GUI and post-game review.

Public schema artifacts live under `schemas/`:

- `schemas/strategy_memory.schema.json`
- `schemas/move_decision.schema.json`
