# Anthropic Provider Setup

Use this guide for Noema64's built-in `anthropic` provider.

## Provider Behavior

Noema64 sends Messages API requests to:

```text
https://api.anthropic.com/v1/messages
```

If you leave Endpoint empty, Noema64 uses `https://api.anthropic.com` as the base URL and appends `/v1/messages`.

The request uses:

- `X-API-Key: <api key>`;
- `Anthropic-Version: 2023-06-01`;
- the configured `model`;
- the configured `temperature` and `max_tokens`;
- Noema64's system prompt as the Messages API `system` field;
- Noema64's current game prompt as the user message.

Unlike OpenAI and Gemini, this adapter does not currently use provider-native structured-output enforcement. It relies on the prompt and the parser. Choose a strong instruction-following model and keep temperature low.

## Prerequisites

1. Create or sign in to an Anthropic Console account.
2. Create an API key.
3. Confirm the account has access to the model ID you plan to use.
4. Use the exact API model ID, not a marketing/display name.

Anthropic documents that API requests must include an `anthropic-version` header, with `2023-06-01` as the version Noema64 currently sends.

## GUI Setup

1. Open Settings.
2. Set Profile to `anthropic-cloud` or select Provider `Anthropic`.
3. Leave Endpoint empty for Anthropic's default API, or enter a compatible Anthropic base URL. Do not include `/v1/messages`.
4. Set Model to the exact Anthropic model ID available to your account.
5. Set Temperature to `0.2`; use `0` if health checks return prose or fenced JSON.
6. Set Max tokens to `2000`.
7. Set LLM timeout ms to `20000`.
8. Enter the API key in `API key`, or use `API key ref`.
9. Check the cloud-provider acknowledgement.
10. Click `Save`, or click `Keychain` if you are storing the typed key in the macOS keychain.
11. Click `Health`.

To store the key in the macOS keychain through the GUI, select the `anthropic-cloud` profile, enter the key, check the acknowledgement, and click `Keychain`. That action saves settings and writes this ref:

```text
provider/anthropic-cloud
```

## YAML Setup

```yaml
privacy:
  cloud_provider_warning_acknowledged: true

llm:
  profile_id: anthropic-cloud
  provider: anthropic
  endpoint: ""
  model: your-anthropic-model-id
  api_key: ""
  api_key_ref: provider/anthropic-cloud
  temperature: 0.2
  max_tokens: 2000
  timeout_ms: 20000
  retries: 1
```

For environment-backed secret resolution:

```sh
export NOEMA64_KEYCHAIN_PROVIDER_ANTHROPIC_CLOUD="sk-ant-..."
```

Then keep `api_key_ref: provider/anthropic-cloud` in the config.

## Verify End To End

1. After saving settings, click `Health`.
2. Confirm the output includes:

```json
{
  "provider": "anthropic",
  "healthy": true
}
```

3. Start or continue a game.
4. Click `Analyze` or make a move that triggers the engine.
5. Open Decision Trace and verify the provider row says `anthropic` and the configured model.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| Health says the response was not valid JSON and mentions a backtick character | The model likely wrapped JSON in Markdown. Lower temperature, try a stronger model, and keep raw prompt logging off unless debugging. |
| `provider returned HTTP 401` | API key is missing, wrong, expired, or not resolved from `api_key_ref`. |
| `provider returned HTTP 429` | Anthropic rate limit, quota, billing, or access issue. Wait, reduce requests, or switch models. |
| `provider returned no text content` | The response shape did not include usable text. Verify endpoint, model, and account access. |
| Endpoint errors | Endpoint should be the base URL, not the full `/v1/messages` URL. |

## Sources

- [Anthropic get started](https://docs.anthropic.com/en/docs/get-started)
- [Anthropic Messages API](https://docs.anthropic.com/en/api/messages)
- [Anthropic API versioning](https://docs.anthropic.com/en/api/versioning)
