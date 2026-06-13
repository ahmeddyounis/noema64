# Prompting

Prompt protocol v1 keeps layers explicit:

1. System role and JSON-only contract.
2. Engine mechanics and legal move whitelist.
3. Position data.
4. Deterministic features.
5. Previous strategy memory.
6. Mode and personality.
7. Output schema.

Imported PGN/comments are treated as untrusted chess data, not instructions.

Personality is rendered as structured profile JSON with `id`, `name`, `risk_tolerance`, strategic biases, and prompt modifiers, so style presets affect the strategist context instead of acting as display-only labels.

Versioned editable templates live under `prompts/v1`:

- `system.md`
- `move_decision.md`
- `schema.json`

At runtime, set `NOEMA64_PROMPT_DIR=/path/to/templates` to use an edited template set without changing code. The directory must contain all three files, and unknown `{{placeholder}}` tokens are rejected.
