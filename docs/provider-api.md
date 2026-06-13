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

The app service exposes provider profiles to the GUI for:

- Health dashboard rows with provider capabilities and status.
- Provider comparison across the deterministic position suite.
- YAML import/export of configured profiles.

Profile export redacts API keys. Import preserves an existing secret when the imported profile contains `[REDACTED]` for the same profile ID. OpenAI-compatible health checks and provider comparison are skipped until the cloud-provider acknowledgement is enabled, because those actions can transmit FEN, legal moves, move history, strategy memory, and selected settings to the configured endpoint.
