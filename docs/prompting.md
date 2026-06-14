# Prompting

Prompt protocol v1 keeps layers explicit:

1. System role and JSON-only contract.
2. Engine mechanics and legal move whitelist.
3. Position data.
4. Structured game context: game ID, ply, move number, side to move, variant metadata, and clock state when a clock is active.
5. Deterministic features.
6. Previous strategy memory.
7. Mode and personality.
8. Output schema.

Imported PGN is treated as untrusted chess data, not instructions. PGN comments are stripped before provider calls so imported comments or prior Noema64 plan annotations cannot steer the LLM as active instructions.

Personality is rendered as structured profile JSON with `id`, `name`, `risk_tolerance`, strategic biases, and prompt modifiers, so style presets affect the strategist context instead of acting as display-only labels.

The selected profile also contributes a bounded `personality_score` to each candidate during deterministic scoring. This keeps personality influence visible in traces and subordinate to legality, verifier, and search signals.

Versioned editable templates live under `prompts/v1`:

- `manifest.json`
- `system.md`
- `move_decision.md`
- `schema.json`

`manifest.json` declares the prompt template schema version, prompt ID, prompt version, app version, and the decision-output schema version targeted by the pack. At runtime, set `NOEMA64_PROMPT_DIR=/path/to/templates` to use an edited template set without changing code. The directory must contain all four files, unknown `{{placeholder}}` tokens are rejected, and incompatible manifest or output schema versions fail before a provider call is made.

The GUI prompt editor loads the active prompt pack, validates manifest/schema/template placeholders against the same runtime checks, and can save the pack back to a chosen local directory. Saving a pack does not automatically switch runtime prompts; launch with `NOEMA64_PROMPT_DIR` pointing at the saved directory to use it.

Decision traces record `prompt_id`, `prompt_version`, `prompt_schema_version`, and `decision_schema_version` for replay and audit tooling.
