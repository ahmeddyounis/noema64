# Security

Report security issues privately to the maintainers before public disclosure.

Relevant areas:

- API key handling and redaction.
- OS keychain references and config-file permission regressions.
- Prompt logging and trace export.
- Imported PGN/FEN content.
- External verifier binary configuration.
- UCI command parsing.

Noema64 is local-first. It does not upload games, prompts, or telemetry by default.

Imported FEN is capped at 512 bytes and imported PGN is capped at 1 MiB in the MVP app service. Imported chess text is treated as untrusted prompt context.

Provider keys can be stored directly in the local config file or referenced with `api_key_ref`. macOS builds can store refs in the system keychain under service `noema64`; unsupported platforms keep the config-file path explicit and permission-restricted.

Release artifact checks are available with `make release-check`. The check validates built artifact presence, emits SHA-256 checksums, and verifies macOS code signatures when available. Set `NOEMA64_REQUIRE_SIGNATURE=1` to fail release checks if signature verification is unavailable or invalid.
