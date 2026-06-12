package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
