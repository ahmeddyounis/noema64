# GUI

The GUI entrypoint is `cmd/noema64-gui` and targets Wails v2.

It exposes:

- Interactive board with drag/drop and click source/destination moves.
- Legal move highlighting from backend data.
- New game, time control, recent games, engine move, analysis-only decision, stop, resign, undo, flip, PGN/FEN/JSONL trace export, and settings controls.
- Settings for mode, personality, provider, API endpoint/key, temperature, max tokens, timeout, retries, cloud-provider acknowledgement, verifier path/timing, external tablebase probe, trace logging, raw logging flags, and log output directory.
- Relaunch restore for the latest saved game, clock, and strategy memory from `logging.output_dir/games`.
- Strategy memory panel.
- Candidate move panel with confidence, plan alignment, final rank, and verifier status.
- Decision trace tabs, prompt/raw-response/parsed-provider inspector, and backend-emitted `decision.stage` progress updates.
- Provider health, random benchmark, and mode benchmark actions.

The analysis button runs the same decision pipeline on the current imported or played position without applying the selected move to the board, clock, move history, or persistent strategy memory. The returned decision trace is marked `analysis_only`.

The frontend never calculates legal chess moves authoritatively.

Run the dependency-free GUI smoke check with:

```sh
npm --prefix cmd/noema64-gui/frontend test
```
