# GUI

The GUI entrypoint is `cmd/noema64-gui` and targets Wails v2.

It exposes:

- Interactive board with click source/destination moves.
- Legal move highlighting from backend data.
- New game, engine move, stop, undo, flip, export, and settings controls.
- Strategy memory panel.
- Candidate move panel.
- Decision trace tabs.
- Provider health and random benchmark actions.

The frontend never calculates legal chess moves authoritatively.

