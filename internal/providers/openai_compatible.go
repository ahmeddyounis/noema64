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
	BaseURL             string
	APIKey              string
	Model               string
	Retries             int
	Client              *http.Client
	OpenAICompatibility bool
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
		System:      "Return exactly one JSON object. Do not wrap it in Markdown.",
		User:        `{"ok":true}`,
		MaxTokens:   16,
		Temperature: 0,
	}
	resp, err := p.CompleteJSON(ctx, req)
	if err != nil {
		return err
	}
	return validateProviderHealthJSON(resp.Text)
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
	requestShape := openAIChatRequestShapeFor(model, baseURL, p.OpenAICompatibility)
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		body := requestShape.body(req, model)
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
		resp, err = client.Do(httpReq)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 400 {
			break
		}
		errorBody, truncated, readErr := readProviderErrorBody(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		lastErr = providerHTTPErrorFromBytes(resp.StatusCode, errorBody, truncated)
		if resp.StatusCode != http.StatusBadRequest || !requestShape.adaptToUnsupportedParameter(errorBody) {
			return nil, lastErr
		}
		resp = nil
	}
	if resp == nil {
		return nil, lastErr
	}
	defer resp.Body.Close()
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

type openAIChatRequestShape struct {
	messageRole           string
	tokenLimitParameter   string
	includeTemperature    bool
	includeResponseFormat bool
}

func openAIChatRequestShapeFor(model string, baseURL string, forceOpenAICompatibility bool) openAIChatRequestShape {
	openAICompatibility := forceOpenAICompatibility || isOpenAIBaseURL(baseURL)
	shape := openAIChatRequestShape{
		messageRole:           "system",
		tokenLimitParameter:   "max_tokens",
		includeTemperature:    true,
		includeResponseFormat: true,
	}
	if !openAICompatibility {
		return shape
	}
	if openAIUsesDeveloperRole(model) {
		shape.messageRole = "developer"
	}
	if openAIUsesMaxCompletionTokens(model) {
		shape.tokenLimitParameter = "max_completion_tokens"
	}
	return shape
}

func (s openAIChatRequestShape) body(req CompletionRequest, model string) map[string]any {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": s.messageRole, "content": req.System},
			{"role": "user", "content": req.User},
		},
	}
	if s.includeTemperature {
		body["temperature"] = req.Temperature
	}
	if s.includeResponseFormat {
		body["response_format"] = map[string]string{
			"type": "json_object",
		}
	}
	body[s.tokenLimitParameter] = req.MaxTokens
	return body
}

func (s *openAIChatRequestShape) adaptToUnsupportedParameter(body []byte) bool {
	param, message, ok := openAIUnsupportedRequestField(body)
	if !ok {
		return false
	}
	switch param {
	case "max_tokens":
		if s.tokenLimitParameter != "max_completion_tokens" {
			s.tokenLimitParameter = "max_completion_tokens"
			return true
		}
	case "max_completion_tokens":
		if s.tokenLimitParameter != "max_tokens" {
			s.tokenLimitParameter = "max_tokens"
			return true
		}
	case "temperature":
		if s.includeTemperature {
			s.includeTemperature = false
			return true
		}
	case "response_format":
		if s.includeResponseFormat {
			s.includeResponseFormat = false
			return true
		}
	case "messages[0].role", "messages.0.role":
		if s.messageRole == "system" && strings.Contains(strings.ToLower(message), "developer") {
			s.messageRole = "developer"
			return true
		}
	}
	if s.messageRole == "system" && strings.Contains(strings.ToLower(message), "use 'developer'") {
		s.messageRole = "developer"
		return true
	}
	return false
}

func openAIUnsupportedRequestField(body []byte) (param string, message string, ok bool) {
	var decoded struct {
		Error struct {
			Message string `json:"message"`
			Param   string `json:"param"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		message = decoded.Error.Message
		param = strings.Trim(decoded.Error.Param, "'\" ")
		code := strings.ToLower(decoded.Error.Code)
		lowerMessage := strings.ToLower(message)
		if code == "unsupported_parameter" || code == "unsupported_value" || strings.Contains(lowerMessage, "unsupported parameter") || strings.Contains(lowerMessage, "unsupported value") {
			if param == "" {
				param = unsupportedParameterFromMessage(message)
			}
			return param, message, param != ""
		}
	}
	message = string(body)
	lowerMessage := strings.ToLower(message)
	if !strings.Contains(lowerMessage, "unsupported") && !strings.Contains(lowerMessage, "not supported") {
		return "", message, false
	}
	param = unsupportedParameterFromMessage(message)
	return param, message, param != ""
}

func unsupportedParameterFromMessage(message string) string {
	lower := strings.ToLower(message)
	for _, marker := range []string{"unsupported parameter: '", "unsupported value: '", "parameter: '"} {
		if start := strings.Index(lower, marker); start >= 0 {
			rest := message[start+len(marker):]
			if end := strings.Index(rest, "'"); end >= 0 {
				return strings.TrimSpace(rest[:end])
			}
		}
	}
	for _, candidate := range []string{"max_completion_tokens", "max_tokens", "temperature", "response_format", "messages[0].role", "messages.0.role"} {
		if strings.Contains(lower, candidate) {
			return candidate
		}
	}
	return ""
}

func openAIUsesDeveloperRole(model string) bool {
	return openAIReasoningStyleModel(model)
}

func openAIUsesMaxCompletionTokens(model string) bool {
	return openAIReasoningStyleModel(model)
}

func openAIReasoningStyleModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	for _, prefix := range []string{"gpt-5", "o1", "o3", "o4"} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func isOpenAIBaseURL(baseURL string) bool {
	normalized := strings.ToLower(strings.TrimSpace(baseURL))
	return strings.HasPrefix(normalized, "https://api.openai.com/") || normalized == "https://api.openai.com"
}
