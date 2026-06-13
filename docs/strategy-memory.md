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

Study tools also expose:

- Compressed memory with a source hash and bounded critical targets, commitments, warnings, triggers, and style notes.
- Plan coherence scoring for missing phase, targets, commitments, triggers, and confidence gaps.
- Candidate diversity scoring for move-family, destination, and purpose variety.
- Deterministic multi-agent review roles: strategist, critic, tactician, and arbiter.
- An editable strategy-memory JSON path through the app service and GUI Study dialog.
- Opening-book suggestions and endgame trainer drills in the Study dashboard; these are advisory surfaces and do not override legal move generation.

Public schema artifacts live under `schemas/`:

- `schemas/strategy_memory.schema.json`
- `schemas/move_decision.schema.json`
