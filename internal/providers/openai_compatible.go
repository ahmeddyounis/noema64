package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatible struct {
	BaseURL string
	APIKey  string
	Model   string
	Retries int
	Client  *http.Client
}

func (p OpenAICompatible) Name() string {
	return "openai_compatible"
}

func (p OpenAICompatible) Capabilities() Capabilities {
	return Capabilities{
		SupportsJSONMode:     true,
		SupportsCancellation: true,
		MaxContextTokens:     128000,
		RecommendedMaxOutput: 1600,
	}
}

func (p OpenAICompatible) HealthCheck(ctx context.Context) error {
	model := p.Model
	if model == "" {
		model = "local-model"
	}
	req := CompletionRequest{
		Model:       model,
		System:      "Return JSON.",
		User:        `{"ok":true}`,
		MaxTokens:   16,
		Temperature: 0,
	}
	resp, err := p.CompleteJSON(ctx, req)
	if err != nil {
		return err
	}
	var parsed any
	if err := json.Unmarshal([]byte(resp.Text), &parsed); err != nil {
		return fmt.Errorf("provider health response was not valid JSON: %w", err)
	}
	return nil
}

func (p OpenAICompatible) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	attempts := p.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
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

func (p OpenAICompatible) completeJSONOnce(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(p.BaseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("provider endpoint is empty")
	}
	model := firstNonEmpty(req.Model, p.Model)
	if model == "" {
		return nil, fmt.Errorf("provider model is empty")
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, providerHTTPError(resp.StatusCode, resp.Body)
	}
	var decoded struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := decodeProviderResponse(resp.Body, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("provider returned no choices")
	}
	text := decoded.Choices[0].Message.Content
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("provider returned empty message")
	}
	return &CompletionResponse{
		Text:         text,
		Provider:     p.Name(),
		Model:        firstNonEmpty(decoded.Model, model),
		Latency:      time.Since(start),
		RawAvailable: true,
	}, nil
}
