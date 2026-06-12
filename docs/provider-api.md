# Provider API

Providers implement:

```go
type Provider interface {
    Name() string
    Capabilities() Capabilities
    CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    HealthCheck(ctx context.Context) error
}
```

The strategy layer owns prompt construction, schema parsing, repair, and fallback. Provider adapters should only turn a request into model output.

Default providers:

- `mock`: deterministic, offline, CI-safe.
- `openai_compatible`: HTTP chat completions adapter for local or cloud endpoints.

Provider responses are untrusted. Invalid JSON, illegal moves, empty output, and timeout all degrade to legal fallback.

