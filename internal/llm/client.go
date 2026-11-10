package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`

	// Agentic providers (claude_subscription) run their own tools instead of Tools:
	// WorkDir is the workspace they operate in, AllowedCommands the shell commands they may run.
	WorkDir         string   `json:"-"`
	AllowedCommands []string `json:"-"`
	// CommandEnv is the environment those commands may see ("NAME=value").
	// The provider's own process is started with it, plus what it needs to
	// log in, since its commands inherit whatever it has. Nil keeps the caller's.
	CommandEnv []string `json:"-"`

	// OnProgress, when set, receives live activity from agentic providers while
	// the request is still running (each tool call and message), so callers can
	// stream what the worker is doing instead of waiting for the final result.
	OnProgress func(ProgressEvent) `json:"-"`
}

// ProgressEvent is one step of an agentic provider's run.
type ProgressEvent struct {
	Kind   string          // "tool" or "text"
	Tool   string          // tool name for Kind "tool" (Read, Edit, Bash, ...)
	Detail string          // short human summary: a file path, a command, or message text
	Input  json.RawMessage // raw tool input for Kind "tool"
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"` // all input processed, including cache reads and writes
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// CachedTokens is the part of PromptTokens served from the prompt cache
	CachedTokens int `json:"cached_tokens,omitempty"`
	// CostUSD is the provider-reported cost of the request, when it reports one
	CostUSD float64 `json:"cost_usd,omitempty"`
}

type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	FinishReason string     `json:"finish_reason"`
	Usage        TokenUsage `json:"usage"`
}

type ModelInfo struct {
	ID   string `json:"id"`
	Desc string `json:"desc,omitempty"`
}

type Provider interface {
	Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

type OpenAICompatibleClient struct {
	BaseURL    string
	APIKeyEnv  string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKeyEnv string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKeyEnv:  apiKeyEnv,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func NewProvider(providerType, baseURL, apiKeyEnv string) Provider {
	switch strings.ToLower(providerType) {
	case "anthropic":
		return NewAnthropicClient(baseURL, apiKeyEnv)
	case "claude_subscription", "claude_sub", "claude-subscription":
		return NewClaudeSubscriptionClient()
	default:
		return NewClient(baseURL, apiKeyEnv)
	}
}

func (c *OpenAICompatibleClient) getAPIKey() (string, error) {
	if c.APIKeyEnv == "" {
		return "", nil
	}
	// If the string starts with standard sk- prefix, treat it as the raw literal key directly
	if strings.HasPrefix(c.APIKeyEnv, "sk-") {
		return c.APIKeyEnv, nil
	}
	key := os.Getenv(c.APIKeyEnv)
	if key == "" {
		// Fallback: check if APIKeyEnv itself was passed as the literal key
		if len(c.APIKeyEnv) > 10 && !strings.Contains(c.APIKeyEnv, " ") {
			return c.APIKeyEnv, nil
		}
		return "", fmt.Errorf("environment variable '%s' for provider API key is not set or empty", c.APIKeyEnv)
	}
	return key, nil
}

func (c *OpenAICompatibleClient) Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, err
	}

	req.Model = model
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.BaseURL)

	var lastErr error
	maxRetries := 2
	backoff := 500 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		// Don't retry client errors (400, 401, 403, 404)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, fmt.Errorf("LLM provider returned client error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}

		// Retry 429 and 5xx
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("LLM provider transient error (HTTP %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		// Parse standard OpenAI response
		var rawResp struct {
			Choices []struct {
				Message struct {
					Content   *string    `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(respBody, &rawResp); err != nil {
			return nil, fmt.Errorf("failed to decode LLM response: %w (body: %s)", err, string(respBody))
		}

		if len(rawResp.Choices) == 0 {
			return nil, fmt.Errorf("no choices returned in response from model: %s", string(respBody))
		}

		choice := rawResp.Choices[0]
		content := ""
		if choice.Message.Content != nil {
			content = *choice.Message.Content
		}

		return &ChatResponse{
			Content:      content,
			ToolCalls:    choice.Message.ToolCalls,
			FinishReason: choice.FinishReason,
			Usage: TokenUsage{
				PromptTokens:     rawResp.Usage.PromptTokens,
				CompletionTokens: rawResp.Usage.CompletionTokens,
				TotalTokens:      rawResp.Usage.TotalTokens,
			},
		}, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (c *OpenAICompatibleClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, _ := c.getAPIKey()
	endpoint := fmt.Sprintf("%s/models", c.BaseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create models request: %w", err)
	}

	if apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	var models []ModelInfo
	for _, m := range raw.Data {
		if m.ID != "" {
			models = append(models, ModelInfo{ID: m.ID})
		}
	}
	return models, nil
}
