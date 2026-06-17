package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleHealthCheckRequiresJSONResponse(t *testing.T) {
	server := openAITestServer(t, `{"ok":true}`, nil)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "test-model", APIKey: "secret"}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
}

func TestOpenAICompatibleHealthCheckAcceptsFencedJSON(t *testing.T) {
	server := openAITestServer(t, "```json\n{\"ok\":true}\n```", nil)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "test-model"}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
}

func TestOpenAICompatibleHealthCheckRejectsNonJSONContent(t *testing.T) {
	server := openAITestServer(t, `plain text`, nil)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "test-model"}
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected non-JSON health response to fail")
	}
}

func TestOpenAICompatibleCompleteJSONRequestShape(t *testing.T) {
	var seen struct {
		Model          string         `json:"model"`
		Messages       []chatMessage  `json:"messages"`
		Temperature    float64        `json:"temperature"`
		MaxTokens      int            `json:"max_tokens"`
		ResponseFormat map[string]any `json:"response_format"`
		Authorization  string
	}
	server := openAITestServer(t, `{"move":"e2e4"}`, &seen)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, APIKey: "secret"}
	resp, err := provider.CompleteJSON(context.Background(), CompletionRequest{
		Model:       "model-a",
		System:      "system",
		User:        "user",
		Temperature: 0.25,
		MaxTokens:   32,
	})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if resp.Text != `{"move":"e2e4"}` || resp.Provider != "openai_compatible" || !resp.RawAvailable {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if seen.Model != "model-a" || seen.Temperature != 0.25 || seen.MaxTokens != 32 {
		t.Fatalf("request scalar fields = %+v", seen)
	}
	if len(seen.Messages) != 2 || seen.Messages[0].Role != "system" || seen.Messages[0].Content != "system" || seen.Messages[1].Role != "user" || seen.Messages[1].Content != "user" {
		t.Fatalf("request messages = %+v", seen.Messages)
	}
	if seen.ResponseFormat["type"] != "json_object" {
		t.Fatalf("response format = %+v", seen.ResponseFormat)
	}
	if seen.Authorization != "Bearer secret" {
		t.Fatalf("authorization = %q", seen.Authorization)
	}
}

func TestOpenAICompatibleUsesMaxCompletionTokensForGPT5Models(t *testing.T) {
	var seen map[string]any
	server := openAITestServer(t, `{"move":"e2e4"}`, &seen)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, APIKey: "secret"}
	if _, err := provider.CompleteJSON(context.Background(), CompletionRequest{
		Model:       "gpt-5.5",
		System:      "system",
		User:        "user",
		Temperature: 0.25,
		MaxTokens:   64,
	}); err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if _, ok := seen["max_tokens"]; ok {
		t.Fatalf("request included max_tokens for GPT-5 model: %+v", seen)
	}
	if got := seen["max_completion_tokens"]; got != float64(64) {
		t.Fatalf("max_completion_tokens = %#v, want 64", got)
	}
}

func TestOpenAIProviderHealthCheckUsesMaxCompletionTokensForGPT5Models(t *testing.T) {
	var seen map[string]any
	server := openAITestServer(t, `{"ok":true}`, &seen)
	defer server.Close()

	provider := OpenAIProvider{BaseURL: server.URL, Model: "gpt-5.5"}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
	if _, ok := seen["max_tokens"]; ok {
		t.Fatalf("health check request included max_tokens for GPT-5 model: %+v", seen)
	}
	if got := seen["max_completion_tokens"]; got != float64(16) {
		t.Fatalf("max_completion_tokens = %#v, want 16", got)
	}
}

func TestOpenAIProviderWrapsCompatibleEndpointWithOpenAIName(t *testing.T) {
	server := openAITestServer(t, `{"move":"e2e4"}`, nil)
	defer server.Close()

	provider := OpenAIProvider{BaseURL: server.URL, Model: "model-a"}
	resp, err := provider.CompleteJSON(context.Background(), CompletionRequest{MaxTokens: 16})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if resp.Provider != "openai" || resp.Text != `{"move":"e2e4"}` {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOpenAICompatibleUsesConfiguredModelWhenRequestModelEmpty(t *testing.T) {
	var seen struct {
		Model string `json:"model"`
	}
	server := openAITestServer(t, `{"move":"e2e4"}`, &seen)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "configured-model"}
	resp, err := provider.CompleteJSON(context.Background(), CompletionRequest{MaxTokens: 16})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if seen.Model != "configured-model" {
		t.Fatalf("request model = %q, want configured-model", seen.Model)
	}
	if resp.Model != "configured-model" {
		t.Fatalf("response model = %q, want configured-model", resp.Model)
	}
}

func TestOpenAICompatibleRejectsEmptyResponseContent(t *testing.T) {
	server := openAITestServer(t, "   ", nil)
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "configured-model"}
	if _, err := provider.CompleteJSON(context.Background(), CompletionRequest{MaxTokens: 16}); err == nil {
		t.Fatal("expected empty provider content to fail")
	}
}

func TestOpenAICompatibleIncludesRedactedHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"You exceeded your current quota.","type":"insufficient_quota","api_key":"sk-testsecret1234567890"}}`))
	}))
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "model-a"}
	_, err := provider.CompleteJSON(context.Background(), CompletionRequest{MaxTokens: 16})
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	text := err.Error()
	if !strings.Contains(text, "provider returned HTTP 429") || !strings.Contains(text, "insufficient_quota") {
		t.Fatalf("error = %q, want HTTP status and provider detail", text)
	}
	if strings.Contains(text, "sk-testsecret") {
		t.Fatalf("error leaked secret: %q", text)
	}
}

func TestOpenAICompatibleRetriesTransientFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": `{"ok":true}`},
			}},
		})
	}))
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Retries: 1}
	resp, err := provider.CompleteJSON(context.Background(), CompletionRequest{Model: "model-a", MaxTokens: 16})
	if err != nil {
		t.Fatalf("complete json with retry: %v", err)
	}
	if attempts != 2 || resp.Text != `{"ok":true}` {
		t.Fatalf("attempts=%d response=%+v", attempts, resp)
	}
}

func TestOpenAICompatibleRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": strings.Repeat("x", maxProviderResponseBytes)},
			}},
		})
	}))
	defer server.Close()

	provider := OpenAICompatible{BaseURL: server.URL, Model: "model-a"}
	_, err := provider.CompleteJSON(context.Background(), CompletionRequest{MaxTokens: 16})
	if err == nil || !strings.Contains(err.Error(), "provider response exceeds") {
		t.Fatalf("error = %v, want oversized response failure", err)
	}
}

func TestProviderDecoderRejectsMultipleJSONValues(t *testing.T) {
	var decoded map[string]any
	err := decodeProviderResponse(strings.NewReader(`{"ok":true} {"extra":true}`), &decoded)
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("error = %v, want multiple JSON values failure", err)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func openAITestServer(t *testing.T, content string, seen any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if seen != nil {
			if err := json.NewDecoder(r.Body).Decode(seen); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if target, ok := seen.(*struct {
				Model          string         `json:"model"`
				Messages       []chatMessage  `json:"messages"`
				Temperature    float64        `json:"temperature"`
				MaxTokens      int            `json:"max_tokens"`
				ResponseFormat map[string]any `json:"response_format"`
				Authorization  string
			}); ok {
				target.Authorization = r.Header.Get("Authorization")
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": content},
			}},
		})
	}))
}
