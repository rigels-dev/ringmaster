package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAnthropicChatSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "secret-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			http.Error(w, "missing version", http.StatusBadRequest)
			return
		}

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Anthropic pong"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  15,
				"output_tokens": 8,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	os.Setenv("TEST_ANTHROPIC_KEY", "secret-test-key")
	defer os.Unsetenv("TEST_ANTHROPIC_KEY")

	client := NewAnthropicClient(ts.URL, "TEST_ANTHROPIC_KEY")
	resp, err := client.Chat(context.Background(), "claude-3-5-sonnet", ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "ping"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Anthropic pong" {
		t.Errorf("expected 'Anthropic pong', got '%s'", resp.Content)
	}
	if resp.Usage.PromptTokens != 15 || resp.Usage.CompletionTokens != 8 {
		t.Errorf("unexpected usage tokens: %+v", resp.Usage)
	}
}

func TestAnthropicToolCalling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{
					"type": "tool_use",
					"id":   "toolu_123",
					"name": "read_file",
					"input": map[string]any{
						"path": "main.go",
					},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]int{
				"input_tokens":  20,
				"output_tokens": 12,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewAnthropicClient(ts.URL, "")
	resp, err := client.Chat(context.Background(), "claude-3-5-sonnet", ChatRequest{
		Messages: []Message{{Role: "user", Content: "Read main.go"}},
		Tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "read_file",
					Description: "read file contents",
					Parameters:  map[string]any{"type": "object"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected read_file, got %s", resp.ToolCalls[0].Function.Name)
	}
}
