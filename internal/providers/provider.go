package providers

import (
	"context"
	"time"
)

type Capabilities struct {
	SupportsJSONMode     bool `json:"supports_json_mode"`
	SupportsStreaming    bool `json:"supports_streaming"`
	SupportsCancellation bool `json:"supports_cancellation"`
	SupportsSeed         bool `json:"supports_seed"`
	SupportsToolCalls    bool `json:"supports_tool_calls"`
	MaxContextTokens     int  `json:"max_context_tokens"`
	RecommendedMaxOutput int  `json:"recommended_max_output"`
}

type CompletionRequest struct {
	Model       string            `json:"model"`
	System      string            `json:"system"`
	User        string            `json:"user"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type CompletionResponse struct {
	Text         string            `json:"text"`
	Provider     string            `json:"provider"`
	Model        string            `json:"model"`
	Latency      time.Duration     `json:"latency"`
	RawAvailable bool              `json:"raw_available"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	CompleteJSON(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	HealthCheck(ctx context.Context) error
}
