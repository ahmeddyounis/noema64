# Licensing

Noema64 source is MIT licensed.

Bundled dependencies are intended to be permissively compatible. External verifier binaries such as Stockfish and external tablebase probes are not bundled in the MVP. Users may configure their own verifier or tablebase path, and that external program retains its own license.

Before public release, run a dependency license review and document any bundled non-permissive dependency.

Local dependency posture check:

```sh
make license-check
```

The check scans downloaded module license files and fails if obvious GPL or AGPL license text appears in bundled dependencies. It is a release guardrail, not a substitute for legal review.
