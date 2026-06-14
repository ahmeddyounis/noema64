package providers

import (
	"context"
	"net/http"
	"strings"
)

const OpenAIBaseURL = "https://api.openai.com/v1"

type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Retries int
	Client  *http.Client
}

func (p OpenAIProvider) Name() string {
	return "openai"
}

func (p OpenAIProvider) Capabilities() Capabilities {
	return p.compatible().Capabilities()
}

func (p OpenAIProvider) HealthCheck(ctx context.Context) error {
	return p.compatible().HealthCheck(ctx)
}

func (p OpenAIProvider) CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	resp, err := p.compatible().CompleteJSON(ctx, req)
	if resp != nil {
		resp.Provider = p.Name()
	}
	return resp, err
}

func (p OpenAIProvider) compatible() OpenAICompatible {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = OpenAIBaseURL
	}
	return OpenAICompatible{
		BaseURL: baseURL,
		APIKey:  p.APIKey,
		Model:   p.Model,
		Retries: p.Retries,
		Client:  p.Client,
	}
}
