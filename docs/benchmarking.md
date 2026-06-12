# Benchmarking

Run the random legal benchmark:

```sh
go run ./cmd/noema64-bench -games 100
```

The MVP pass bar is 100 completed random-opponent games with zero illegal final moves. The benchmark uses the mock provider by default and exercises the real engine pipeline, fallback, memory merge, and legal move application. Random games that reach the configured ply cap are recorded as `adjudicated_draw` so the benchmark always returns a terminal result record.
