# Privacy

Noema64 is local-first and does not upload telemetry by default.

Mock provider mode is fully offline. Endpoint-backed provider modes may send the current FEN, legal moves, move history, strategy memory, selected settings, and optional user notes to the configured provider, including OpenAI, local or remote OpenAI-compatible endpoints, and Ollama endpoints.

Raw prompt logging is disabled by default. API keys are redacted from traces and logs. Normal JSONL trace export strips raw prompts and raw LLM responses even if they were logged. Full debug trace export keeps raw prompt/response fields and requires explicit GUI confirmation.

Fine-tune dataset export is local-only. It derives JSONL examples from sanitized decision traces or the current in-memory decision and does not upload data. Review exported examples before using them with any external training provider.

Backup archives are created locally as zip files containing config/log/game artifacts and a manifest. Restore extracts only safe relative paths into the selected target directory.

Provider settings support either a local `api_key` or an `api_key_ref`. On macOS, the GUI can store a typed provider key in the OS keychain under service `noema64` and replace the config value with a reference. On other platforms, keychain storage reports unsupported and the config file remains permission-restricted to `0600`.

GUI game snapshots are local JSON files under the configured log directory. They include board state, move history, strategy memory, and the last decision payload; the snapshot writer redacts secrets before saving.
