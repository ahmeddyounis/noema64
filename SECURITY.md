# Security

Report security issues privately to the maintainers before public disclosure.

Relevant areas:

- API key handling and redaction.
- Prompt logging and trace export.
- Imported PGN/FEN content.
- External verifier binary configuration.
- UCI command parsing.

Noema64 is local-first. It does not upload games, prompts, or telemetry by default.

Imported FEN is capped at 512 bytes and imported PGN is capped at 1 MiB in the MVP app service. Imported chess text is treated as untrusted prompt context.
