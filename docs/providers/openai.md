# OpenAI Provider Setup

Use this guide for Noema64's built-in `openai` provider. This is different from `openai_compatible`: the `openai` provider manages the endpoint automatically and uses `https://api.openai.com/v1`.

## Provider Behavior

Noema64 sends Chat Completions requests to:

```text
https://api.openai.com/v1/chat/completions
```

The request uses:

- `Authorization: Bearer <api key>`;
- the configured `model`;
- the configured `temperature` and output token limit;
- `response_format: { "type": "json_object" }`.

Noema64 keeps the user-facing setting name `max_tokens` in YAML and the GUI. For GPT-5 and OpenAI reasoning-model Chat Completions requests, the adapter sends that value as `max_completion_tokens` because those models reject the legacy `max_tokens` request field.

For those same models, Noema64 sends the instruction prompt as a `developer` message instead of a legacy `system` message. If OpenAI or an OpenAI-compatible endpoint returns a narrow `unsupported_parameter` or `unsupported_value` error for `max_tokens`, `max_completion_tokens`, `temperature`, `response_format`, or the first message role, Noema64 retries with an adjusted request shape instead of immediately falling back.

OpenAI's current API docs still document JSON object response format for Chat Completions, while recommending `json_schema` for models that support it. Noema64 currently uses JSON object mode because the same provider pipeline also supports OpenAI-compatible endpoints.

## Prerequisites

1. Create or sign in to an OpenAI platform account.
2. Create an API key from the OpenAI dashboard.
3. Confirm the account has access to the model ID you plan to use.
4. Use a model that supports Chat Completions and JSON object output.

For local smoke testing outside Noema64, OpenAI documents `OPENAI_API_KEY` as the usual environment variable:

```sh
export OPENAI_API_KEY="your_api_key_here"
```

Noema64 does not read `OPENAI_API_KEY` directly. Use the GUI API key field, the GUI Keychain action, or a Noema64 `api_key_ref`.

## GUI Setup

1. Open Settings.
2. Set Profile to `openai-cloud` or select Provider `OpenAI`.
3. Leave Endpoint empty. The GUI disables this field for OpenAI because the app uses the managed OpenAI endpoint.
4. Set Model to the exact API model ID, for example a current OpenAI chat model enabled for your account.
5. Set Temperature to `0.2` for normal play, or `0` while diagnosing JSON-format problems.
6. Set Max tokens to `2000`.
7. Set LLM timeout ms to `20000`.
8. Enter the API key in `API key`, or use `API key ref`.
9. Check the cloud-provider acknowledgement.
10. Click `Save`, or click `Keychain` if you are storing the typed key in the macOS keychain.
11. Click `Health`.

To store the key in the macOS keychain through the GUI, select the `openai-cloud` profile, enter the key, check the acknowledgement, and click `Keychain`. That action saves settings and writes this ref:

```text
provider/openai-cloud
```

## YAML Setup

Use this when editing `config.yaml` directly:

```yaml
privacy:
  cloud_provider_warning_acknowledged: true

llm:
  profile_id: openai-cloud
  provider: openai
  endpoint: ""
  model: your-openai-model-id
  api_key: ""
  api_key_ref: provider/openai-cloud
  temperature: 0.2
  max_tokens: 2000
  timeout_ms: 20000
  retries: 1
```

For environment-backed secret resolution:

```sh
export NOEMA64_KEYCHAIN_PROVIDER_OPENAI_CLOUD="sk-..."
```

Then keep `api_key_ref: provider/openai-cloud` in the config.

## Verify End To End

1. After saving settings, click `Health`.
2. Confirm the output includes:

```json
{
  "provider": "openai",
  "healthy": true
}
```

3. Start or continue a game.
4. Click `Analyze` or make a move that triggers the engine.
5. Open Decision Trace and verify the provider row says `openai` and the configured model.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| OpenAI still looks like mock | Settings were not saved, provider health failed, or the decision fell back. Check Decision Trace -> Provider and Fallback. |
| `provider returned HTTP 401` | API key is missing, wrong, or not resolved from `api_key_ref`. |
| `provider returned HTTP 429` | OpenAI rate limit, quota, billing, or access issue. Wait, reduce requests, or switch models. |
| `Unsupported parameter: 'max_tokens'` | Upgrade to a build that includes the GPT-5 token-limit mapping, then save settings and retry Health. |
| `provider model is empty` | Fill the Model field. |
| Invalid JSON health response | Use a JSON-capable Chat Completions model, lower temperature, and avoid setting OpenAI as `openai_compatible` unless you intentionally need a custom endpoint. |

## Sources

- [OpenAI quickstart](https://developers.openai.com/api/docs/quickstart)
- [OpenAI chat completions response format](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
