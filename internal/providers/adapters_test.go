package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicProviderCompleteJSONRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "secret" || r.Header.Get("Anthropic-Version") == "" {
			t.Fatalf("missing anthropic headers: %+v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["system"] != "system" || body["model"] != "claude-test" {
			t.Fatalf("bad request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-test","content":[{"type":"text","text":"{\"move\":\"e2e4\"}"}]}`))
	}))
	defer server.Close()

	resp, err := (AnthropicProvider{BaseURL: server.URL, APIKey: "secret"}).CompleteJSON(context.Background(), CompletionRequest{
		Model:     "claude-test",
		System:    "system",
		User:      "user",
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if resp.Provider != "anthropic" || resp.Text != `{"move":"e2e4"}` {
		t.Fatalf("bad response: %+v", resp)
	}
}

func TestGeminiProviderCompleteJSONRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models/gemini-test:generateContent") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "secret" {
			t.Fatalf("missing api key query: %s", r.URL.RawQuery)
		}
		var body struct {
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.GenerationConfig["responseMimeType"] != "application/json" {
			t.Fatalf("missing responseMimeType: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"move\":\"d2d4\"}"}]}}]}`))
	}))
	defer server.Close()

	resp, err := (GeminiProvider{BaseURL: server.URL, APIKey: "secret"}).CompleteJSON(context.Background(), CompletionRequest{
		Model:     "gemini-test",
		System:    "system",
		User:      "user",
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if resp.Provider != "gemini" || resp.Text != `{"move":"d2d4"}` {
		t.Fatalf("bad response: %+v", resp)
	}
}

func TestOllamaProviderCompleteJSONRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["format"] != "json" || body["stream"] != false || body["model"] != "llama-test" {
			t.Fatalf("bad request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"llama-test","message":{"content":"{\"move\":\"g1f3\"}"}}`))
	}))
	defer server.Close()

	resp, err := (OllamaProvider{BaseURL: server.URL}).CompleteJSON(context.Background(), CompletionRequest{
		Model:     "llama-test",
		System:    "system",
		User:      "user",
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("complete json: %v", err)
	}
	if resp.Provider != "ollama" || resp.Text != `{"move":"g1f3"}` {
		t.Fatalf("bad response: %+v", resp)
	}
}
