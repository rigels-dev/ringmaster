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

type AnthropicClient struct {
	BaseURL    string
	APIKeyEnv  string
	HTTPClient *http.Client
}

func NewAnthropicClient(baseURL, apiKeyEnv string) *AnthropicClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKeyEnv:  apiKeyEnv,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *AnthropicClient) getAPIKey() (string, error) {
	if c.APIKeyEnv == "" || c.APIKeyEnv == "NONE" {
		return "", nil
	}
	if strings.HasPrefix(c.APIKeyEnv, "sk-") {
		return c.APIKeyEnv, nil
	}
	key := os.Getenv(c.APIKeyEnv)
	if key == "" {
		if len(c.APIKeyEnv) > 10 && !strings.Contains(c.APIKeyEnv, " ") {
			return c.APIKeyEnv, nil
		}
		return "", fmt.Errorf("environment variable '%s' for Anthropic API key is not set or empty", c.APIKeyEnv)
	}
	return key, nil
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
}

func (c *AnthropicClient) Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, err
	}

	// Separate system message from conversation messages
	var systemText string
	var anthropicMsgs []anthropicMessage

	for _, m := range req.Messages {
		if strings.ToLower(m.Role) == "system" {
			if systemText != "" {
				systemText += "\n\n"
			}
			systemText += m.Content
			continue
		}

		if m.ToolCallID != "" {
			// Tool result message
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role: "user",
				Content: []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}

		if len(m.ToolCalls) > 0 {
			// Assistant tool call
			var contentBlocks []map[string]any
			if m.Content != "" {
				contentBlocks = append(contentBlocks, map[string]any{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var inputMap map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
					inputMap = map[string]any{"raw": tc.Function.Arguments}
				}
				contentBlocks = append(contentBlocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": inputMap,
				})
			}
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    "assistant",
				Content: contentBlocks,
			})
			continue
		}

		// Standard user or assistant message
		role := m.Role
		if role != "assistant" {
			role = "user"
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	var tools []anthropicTool
	for _, t := range req.Tools {
		params := t.Function.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: params,
		})
	}

	anthReq := anthropicRequest{
		Model:       model,
		Messages:    anthropicMsgs,
		System:      systemText,
		MaxTokens:   4096,
		Tools:       tools,
		Temperature: req.Temperature,
	}

	bodyBytes, err := json.Marshal(anthReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/messages", c.BaseURL)

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
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
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

		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, fmt.Errorf("Anthropic API error (HTTP %d): %s", resp.StatusCode, string(respBody))
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("Anthropic API transient error (HTTP %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		var rawResp struct {
			Content []struct {
				Type  string         `json:"type"`
				Text  string         `json:"text,omitempty"`
				ID    string         `json:"id,omitempty"`
				Name  string         `json:"name,omitempty"`
				Input map[string]any `json:"input,omitempty"`
			} `json:"content"`
			StopReason string `json:"stop_reason"`
			Usage      struct {
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(respBody, &rawResp); err != nil {
			return nil, fmt.Errorf("failed to decode Anthropic response: %w (body: %s)", err, string(respBody))
		}

		var textContent string
		var toolCalls []ToolCall

		for _, item := range rawResp.Content {
			if item.Type == "text" {
				textContent += item.Text
			} else if item.Type == "tool_use" {
				argBytes, _ := json.Marshal(item.Input)
				toolCalls = append(toolCalls, ToolCall{
					ID:   item.ID,
					Type: "function",
					Function: FunctionCall{
						Name:      item.Name,
						Arguments: string(argBytes),
					},
				})
			}
		}

		return &ChatResponse{
			Content:      textContent,
			ToolCalls:    toolCalls,
			FinishReason: rawResp.StopReason,
			Usage: TokenUsage{
				PromptTokens:     rawResp.Usage.InputTokens + rawResp.Usage.CacheCreationInputTokens + rawResp.Usage.CacheReadInputTokens,
				CompletionTokens: rawResp.Usage.OutputTokens,
				TotalTokens:      rawResp.Usage.InputTokens + rawResp.Usage.CacheCreationInputTokens + rawResp.Usage.CacheReadInputTokens + rawResp.Usage.OutputTokens,
				CachedTokens:     rawResp.Usage.CacheReadInputTokens,
			},
		}, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (c *AnthropicClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, _ := c.getAPIKey()
	endpoint := fmt.Sprintf("%s/models", c.BaseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err == nil {
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
		}
		resp, err := c.HTTPClient.Do(httpReq)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var raw struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil && len(raw.Data) > 0 {
				var models []ModelInfo
				for _, m := range raw.Data {
					models = append(models, ModelInfo{ID: m.ID})
				}
				return models, nil
			}
		}
	}

	return []ModelInfo{
		{ID: "claude-3-7-sonnet-20250219", Desc: "Claude 3.7 Sonnet (Hybrid Reasoning & Coding)"},
		{ID: "claude-3-5-sonnet-20241022", Desc: "Claude 3.5 Sonnet v2 (Coding Benchmark Leader)"},
		{ID: "claude-3-5-haiku-20241022", Desc: "Claude 3.5 Haiku (Fast & Token Efficient)"},
		{ID: "claude-opus-5-5", Desc: "Claude Opus 5.5 (Deep Architecture & Planning)"},
		{ID: "claude-sonnet-5-5", Desc: "Claude Sonnet 5.5 (Next-Gen Balanced Model)"},
		{ID: "claude-haiku-5-5", Desc: "Claude Haiku 5.5 (Ultra-Fast Worker)"},
		{ID: "claude-3-opus-20240229", Desc: "Claude 3 Opus (Complex Logic & Synthesis)"},
	}, nil
}
