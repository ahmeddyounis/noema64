# Gemini Provider Setup

Use this guide for Noema64's built-in `gemini` provider.

## Provider Behavior

Noema64 sends `generateContent` requests to:

```text
https://generativelanguage.googleapis.com/v1beta/models/<model>:generateContent?key=<api key>
```

If you leave Endpoint empty, Noema64 uses `https://generativelanguage.googleapis.com/v1beta` as the base URL and appends `/models/<model>:generateContent`.

The request uses:

- the configured `model`;
- the configured `temperature` and `max_tokens`;
- `generationConfig.responseMimeType: application/json`;
- Noema64's system prompt and current game prompt combined into one user content part.

Noema64 passes the API key in the request URL query string because that is how the current adapter is implemented. The key is still redacted from saved settings and traces.

## Prerequisites

1. Create or sign in to Google AI Studio.
2. Create a Gemini API key.
3. Confirm the key's Google Cloud project has billing/quota configured for your intended usage.
4. Use a Gemini model that supports JSON output through `responseMimeType: application/json`.

Important key timing: Google's Gemini API key docs say unrestricted standard API keys are rejected starting June 19, 2026, and standard keys are rejected starting September 2026. Create a new auth key or restrict older standard keys before relying on Gemini for Noema64 games.

## GUI Setup

1. Open Settings.
2. Set Profile to `gemini-cloud` or select Provider `Gemini`.
3. Leave Endpoint empty for Google's default Gemini API, or enter a compatible base URL. Do not include `/models/...:generateContent`.
4. Set Model to the exact Gemini model ID available to your account.
5. Set Temperature to `0.2`; use `0` if health checks return prose or invalid JSON.
6. Set Max tokens to `2000`.
7. Set LLM timeout ms to `20000`.
8. Enter the API key in `API key`, or use `API key ref`.
9. Check the cloud-provider acknowledgement.
10. Click `Save`, or click `Keychain` if you are storing the typed key in the macOS keychain.
11. Click `Health`.

To store the key in the macOS keychain through the GUI, select the `gemini-cloud` profile, enter the key, check the acknowledgement, and click `Keychain`. That action saves settings and writes this ref:

```text
provider/gemini-cloud
```

## YAML Setup

```yaml
privacy:
  cloud_provider_warning_acknowledged: true

llm:
  profile_id: gemini-cloud
  provider: gemini
  endpoint: ""
  model: your-gemini-model-id
  api_key: ""
  api_key_ref: provider/gemini-cloud
  temperature: 0.2
  max_tokens: 2000
  timeout_ms: 20000
  retries: 1
```

For environment-backed secret resolution:

```sh
export NOEMA64_KEYCHAIN_PROVIDER_GEMINI_CLOUD="..."
```

Then keep `api_key_ref: provider/gemini-cloud` in the config.

## Verify End To End

1. After saving settings, click `Health`.
2. Confirm the output includes:

```json
{
  "provider": "gemini",
  "healthy": true
}
```

3. Start or continue a game.
4. Click `Analyze` or make a move that triggers the engine.
5. Open Decision Trace and verify the provider row says `gemini` and the configured model.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| `provider returned HTTP 400` | Model may not support JSON response mode, endpoint may be malformed, or the key/project may not be enabled for Gemini. |
| `provider returned HTTP 401` or `403` | API key is wrong, restricted incorrectly, expired, or cannot access the selected project/model. |
| `provider returned HTTP 429` | Rate limit, quota, or billing issue. Wait, reduce requests, or use another model/project. |
| Invalid JSON health response | Use a model that supports JSON output and keep temperature low. |
| Endpoint errors | Endpoint should be the base URL, not the full `/models/<model>:generateContent` URL. |

## Sources

- [Gemini API keys](https://ai.google.dev/gemini-api/docs/api-key)
- [Gemini generateContent API](https://ai.google.dev/api/generate-content)
- [Gemini structured output](https://ai.google.dev/gemini-api/docs/structured-output)
