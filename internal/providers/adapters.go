package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/security"
)

const maxProviderResponseBytes = 4 << 20
const maxProviderErrorBytes = 8 << 10

type AnthropicProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Retries int
	Client  *http.Client
}

func (p AnthropicProvider) Name() string {
	return "anthropic"
}

func (p AnthropicProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		MaxContextTokens:     200000,
		RecommendedMaxOutput: 1600,
	}
}

func (p AnthropicProvider) HealthCheck(ctx context.Context) error {
	return healthCheckJSON(ctx, p, p.Model)
}

func (p AnthropicProvider) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	attempts := retryAttempts(p.Retries)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := p.completeJSONOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (p AnthropicProvider) completeJSONOnce(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	baseURL := strings.TrimRight(p.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	body := map[string]any{
		"model":       firstNonEmpty(req.Model, p.Model),
		"system":      req.System,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"messages": []map[string]string{
			{"role": "user", "content": req.User},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Anthropic-Version", "2023-06-01")
	if p.APIKey != "" {
		httpReq.Header.Set("X-API-Key", p.APIKey)
	}
	httpResp, err := httpClient(p.Client).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 400 {
		return nil, providerHTTPError(httpResp.StatusCode, httpResp.Body)
	}
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model string `json:"model"`
	}
	if err := decodeProviderResponse(httpResp.Body, &decoded); err != nil {
		return nil, err
	}
	for _, part := range decoded.Content {
		if part.Text != "" {
			return &CompletionResponse{Text: part.Text, Provider: p.Name(), Model: firstNonEmpty(decoded.Model, req.Model, p.Model), Latency: time.Since(start), RawAvailable: true}, nil
		}
	}
	return nil, fmt.Errorf("provider returned no text content")
}

type GeminiProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Retries int
	Client  *http.Client
}

func (p GeminiProvider) Name() string {
	return "gemini"
}

func (p GeminiProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		MaxContextTokens:     1000000,
		RecommendedMaxOutput: 1600,
	}
}

func (p GeminiProvider) HealthCheck(ctx context.Context) error {
	return healthCheckJSON(ctx, p, p.Model)
}

func (p GeminiProvider) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	attempts := retryAttempts(p.Retries)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := p.completeJSONOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (p GeminiProvider) completeJSONOnce(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	model := firstNonEmpty(req.Model, p.Model, "gemini-1.5-flash")
	endpoint, err := geminiEndpoint(p.BaseURL, model, p.APIKey)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": strings.TrimSpace(req.System + "\n\n" + req.User)},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      req.Temperature,
			"maxOutputTokens":  req.MaxTokens,
			"responseMimeType": "application/json",
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := httpClient(p.Client).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 400 {
		return nil, providerHTTPError(httpResp.StatusCode, httpResp.Body)
	}
	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := decodeProviderResponse(httpResp.Body, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Candidates) == 0 || len(decoded.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("provider returned no candidates")
	}
	return &CompletionResponse{Text: decoded.Candidates[0].Content.Parts[0].Text, Provider: p.Name(), Model: model, Latency: time.Since(start), RawAvailable: true}, nil
}

type OllamaProvider struct {
	BaseURL string
	Model   string
	Retries int
	Client  *http.Client
}

func (p OllamaProvider) Name() string {
	return "ollama"
}

func (p OllamaProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		MaxContextTokens:     128000,
		RecommendedMaxOutput: 1600,
	}
}

func (p OllamaProvider) HealthCheck(ctx context.Context) error {
	return healthCheckJSON(ctx, p, p.Model)
}

func (p OllamaProvider) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	attempts := retryAttempts(p.Retries)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := p.completeJSONOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (p OllamaProvider) completeJSONOnce(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	baseURL := strings.TrimRight(p.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	body := map[string]any{
		"model":  firstNonEmpty(req.Model, p.Model),
		"stream": false,
		"format": "json",
		"messages": []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
		"options": map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := httpClient(p.Client).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 400 {
		return nil, providerHTTPError(httpResp.StatusCode, httpResp.Body)
	}
	var decoded struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Model string `json:"model"`
	}
	if err := decodeProviderResponse(httpResp.Body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Message.Content == "" {
		return nil, fmt.Errorf("provider returned empty message")
	}
	return &CompletionResponse{Text: decoded.Message.Content, Provider: p.Name(), Model: firstNonEmpty(decoded.Model, req.Model, p.Model), Latency: time.Since(start), RawAvailable: true}, nil
}

func retryAttempts(retries int) int {
	attempts := retries + 1
	if attempts < 1 {
		return 1
	}
	return attempts
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func decodeProviderResponse(body io.Reader, target any) error {
	data, err := io.ReadAll(io.LimitReader(body, maxProviderResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxProviderResponseBytes {
		return fmt.Errorf("provider response exceeds %d bytes", maxProviderResponseBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("provider response contained multiple JSON values")
		}
		return err
	}
	return nil
}

func providerHTTPError(statusCode int, body io.Reader) error {
	data, truncated, err := readProviderErrorBody(body)
	if err != nil {
		return fmt.Errorf("provider returned HTTP %d; failed to read error body: %w", statusCode, err)
	}
	return providerHTTPErrorFromBytes(statusCode, data, truncated)
}

func readProviderErrorBody(body io.Reader) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxProviderErrorBytes+1))
	if err != nil {
		return nil, false, err
	}
	truncated := len(data) > maxProviderErrorBytes
	if truncated {
		data = data[:maxProviderErrorBytes]
	}
	return data, truncated, nil
}

func providerHTTPErrorFromBytes(statusCode int, data []byte, truncated bool) error {
	if len(data) == 0 {
		return fmt.Errorf("provider returned HTTP %d", statusCode)
	}
	detail := security.RedactSecrets(strings.Join(strings.Fields(string(data)), " "))
	if detail == "" {
		return fmt.Errorf("provider returned HTTP %d", statusCode)
	}
	if truncated {
		detail += "..."
	}
	return fmt.Errorf("provider returned HTTP %d: %s", statusCode, detail)
}

func healthCheckJSON(ctx context.Context, provider Provider, model string) error {
	resp, err := provider.CompleteJSON(ctx, CompletionRequest{
		Model:       firstNonEmpty(model, "health-check"),
		System:      "Return exactly one JSON object. Do not wrap it in Markdown.",
		User:        `{"ok":true}`,
		MaxTokens:   16,
		Temperature: 0,
	})
	if err != nil {
		return err
	}
	return validateProviderHealthJSON(resp.Text)
}

func validateProviderHealthJSON(text string) error {
	text = strings.TrimSpace(text)
	var parsed any
	directErr := json.Unmarshal([]byte(text), &parsed)
	if directErr == nil {
		return nil
	}
	extracted, err := firstJSONObject(text)
	if err != nil {
		return fmt.Errorf("provider health response was not valid JSON: %w", directErr)
	}
	if err := json.Unmarshal([]byte(extracted), &parsed); err != nil {
		return fmt.Errorf("provider health response was not valid JSON: %w", err)
	}
	return nil
}

func firstJSONObject(s string) (string, error) {
	start := strings.Index(s, "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON object start")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated JSON object")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func geminiEndpoint(baseURL string, model string, apiKey string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	parsed, err := url.Parse(baseURL + "/models/" + url.PathEscape(model) + ":generateContent")
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		q := parsed.Query()
		q.Set("key", apiKey)
		parsed.RawQuery = q.Encode()
	}
	return parsed.String(), nil
}
