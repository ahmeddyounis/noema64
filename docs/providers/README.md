# Provider Setup Guides

These guides cover the cloud providers supported by Noema64's built-in adapters:

- [OpenAI](openai.md)
- [Anthropic](anthropic.md)
- [Gemini](gemini.md)

Noema64 also supports `mock`, `openai_compatible`, `ollama`, and `policy_prior`, but those are local/offline or generic endpoint paths rather than provider-specific cloud integrations.

## What Noema64 Sends

Endpoint-backed providers can receive the current FEN, legal moves, move history, strategy memory, selected settings, clock data, and prompt instructions. Raw prompt and raw response logging are disabled by default. Enable those logs only when you are deliberately debugging provider behavior.

The GUI blocks saving endpoint-backed providers until you acknowledge the cloud-provider data-sharing warning.

## Common GUI Setup

1. Start the app with `wails dev` from `cmd/noema64-gui`, or open a packaged desktop build.
2. Open Settings.
3. Under Provider, choose a profile or select a provider manually.
4. Enter the model ID exactly as the provider exposes it for API use.
5. Enter an API key in `API key`, or enter an `API key ref` that resolves through Noema64's keychain lookup.
6. Check the cloud-provider acknowledgement.
7. Click `Save`. If you are storing the typed key in the macOS keychain, click `Keychain` instead; that action also saves settings.
8. Click `Health` to validate the saved active provider.
9. Make a move or click `Analyze`, then inspect the Decision Trace provider row to confirm the selected provider and model are being used.

If the health check fails, the app may still have settings saved, but gameplay will fall back when the provider cannot return valid JSON and legal move candidates.

## Secrets

Noema64 accepts either a raw `api_key` or an `api_key_ref`.

- `api_key`: saved in the local config file, which Noema64 writes with restricted file permissions.
- `api_key_ref`: resolved first from an environment variable, then from the macOS keychain when available.
- GUI `Keychain`: stores the typed key in the macOS keychain under service `noema64` and writes a `provider/<profile-id>` reference.

For CI or non-macOS automation, use the environment-backed ref form. Non-alphanumeric characters become underscores and the name is uppercased:

```sh
export NOEMA64_KEYCHAIN_PROVIDER_OPENAI_CLOUD="sk-..."
```

```yaml
llm:
  api_key_ref: provider/openai-cloud
```

The same pattern works for `provider/anthropic-cloud` -> `NOEMA64_KEYCHAIN_PROVIDER_ANTHROPIC_CLOUD` and `provider/gemini-cloud` -> `NOEMA64_KEYCHAIN_PROVIDER_GEMINI_CLOUD`.

## Common YAML Fields

Noema64 loads settings from the platform user config directory under `noema64/config.yaml` unless a caller provides a custom settings path. These snippets show only the relevant fields:

```yaml
privacy:
  cloud_provider_warning_acknowledged: true

llm:
  provider: openai
  endpoint: ""
  model: provider-model-id
  api_key: ""
  api_key_ref: provider/openai-cloud
  temperature: 0.2
  max_tokens: 2000
  timeout_ms: 20000
  retries: 1
```

Use a raw `api_key` or an `api_key_ref`, not both. Prefer `api_key_ref` for repeatable setup.

## Health Check Expectations

The `Health` button asks the active provider to return one small JSON object. A healthy provider must:

- authenticate successfully;
- accept the configured model and endpoint;
- return parseable JSON, not Markdown-wrapped JSON;
- complete before the health-check timeout.

Typical failures:

| Error | Likely cause | Fix |
| --- | --- | --- |
| `privacy_ack_required` | Cloud-provider acknowledgement is unchecked | Check the acknowledgement and save. |
| `keychain_unavailable` | `api_key_ref` cannot be resolved | Set the matching `NOEMA64_KEYCHAIN_...` variable or store the key in macOS keychain. |
| `provider returned HTTP 401` | Missing or invalid key | Recreate the key and verify the active profile uses it. |
| `provider returned HTTP 429` | Rate limit, quota, or billing issue | Check provider billing/rate limits, reduce retries, or use another model. |
| `provider health response was not valid JSON` | Model returned prose or fenced JSON | Lower temperature, use a model with reliable JSON output, and avoid unsupported endpoints. |

## Provider Profiles

The `Profiles` button exports/imports provider profiles as YAML. Profiles are useful when switching between OpenAI, Anthropic, Gemini, local endpoints, and mock mode. Export redacts API keys; importing `[REDACTED]` preserves an existing key for the same profile ID.

Example profile shape:

```yaml
profiles:
  - id: openai-cloud
    provider: openai
    mode: quality
    intended_use: openai_cloud_strategy
    model: provider-model-id
    api_key_ref: provider/openai-cloud
    temperature: 0.2
    max_tokens: 2000
    timeout_ms: 20000
    retries: 1
```

## Sources

External provider details were checked on 2026-06-14 against:

- [OpenAI quickstart](https://developers.openai.com/api/docs/quickstart)
- [OpenAI chat completions response format](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [Anthropic get started](https://docs.anthropic.com/en/docs/get-started)
- [Anthropic API versioning](https://docs.anthropic.com/en/api/versioning)
- [Gemini API keys](https://ai.google.dev/gemini-api/docs/api-key)
- [Gemini generateContent API](https://ai.google.dev/api/generate-content)
- [Gemini structured output](https://ai.google.dev/gemini-api/docs/structured-output)
