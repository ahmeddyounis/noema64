# GUI

The GUI entrypoint is `cmd/noema64-gui` and targets Wails v2.

It exposes:

- Interactive board with click source/destination moves.
- Legal move highlighting from backend data.
- New game, engine move, stop, undo, flip, PGN/FEN export, and settings controls.
- Settings for mode, personality, provider, API endpoint/key, verifier path/timing, trace logging, raw logging flags, and log output directory.
- Strategy memory panel.
- Candidate move panel.
- Decision trace tabs.
- Provider health and random benchmark actions.

The frontend never calculates legal chess moves authoritatively.
