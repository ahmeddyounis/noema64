# Privacy

Noema64 is local-first and does not upload telemetry by default.

Mock provider mode is fully offline. Cloud provider mode may send the current FEN, legal moves, move history, strategy memory, selected settings, and optional user notes to the configured provider.

Raw prompt logging is disabled by default. API keys are redacted from traces and logs.

GUI game snapshots are local JSON files under the configured log directory. They include board state, move history, strategy memory, and the last decision payload; the snapshot writer redacts secrets before saving.
