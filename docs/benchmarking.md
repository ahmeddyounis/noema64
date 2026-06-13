# Benchmarking

Run the random legal benchmark:

```sh
go run ./cmd/noema64-bench -games 100
```

The MVP pass bar is 100 completed random-opponent games with zero illegal final moves. The benchmark uses the mock provider by default and exercises the real engine pipeline, fallback, memory merge, and legal move application. Random games that reach the configured ply cap are recorded as `adjudicated_draw` so the benchmark always returns a terminal result record.

Run the mode comparison benchmark:

```sh
go run ./cmd/noema64-bench -modes -games 20
```

The mode benchmark runs pure, blunderguard, and hybrid with the same seed and game count per mode so their completion, fallback, adjudication, and error metrics are directly comparable.

The GUI experiment dashboard also exposes:

- Position suite runs over deterministic FEN positions covering opening development, king safety, tactical tension, and endgame conversion.
- Provider comparison runs the same position suite across configured provider profiles. Cloud or local OpenAI-compatible profiles are skipped until the cloud-provider acknowledgement is enabled, so comparison does not silently transmit game data.
- Provider dashboard health checks with provider capabilities, model, endpoint, timeout, retry, and privacy-gate status.

JSON is the default output format. Use CSV for spreadsheet import or CI artifacts:

```sh
go run ./cmd/noema64-bench -games 100 -format csv > benchmark.csv
go run ./cmd/noema64-bench -modes -games 20 -format csv > mode-benchmark.csv
```

Use `-out` to keep reproducible run artifacts while still printing the requested format to stdout:

```sh
go run ./cmd/noema64-bench -games 100 -seed 64 -out runs/pure_vs_random_001
```

The output directory contains `config.yaml`, `summary.json`, and `summary.csv`.

Validate the current trace schema contract:

```sh
make trace-validate
```

This runs the storage trace tests that assert schema versioning, redaction, strategy memory hashes, candidate scoring components, verifier disclosure, and timing fields.
