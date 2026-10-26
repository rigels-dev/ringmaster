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
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	FinishReason string     `json:"finish_reason"`
	Usage        TokenUsage `json:"usage"`
}

type Provider interface {
	Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error)
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
	key := os.Getenv(c.APIKeyEnv)
	if key == "" {
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
