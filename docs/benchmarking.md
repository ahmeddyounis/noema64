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
