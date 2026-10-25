package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClientChatSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or bad auth header: %s", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Hello from mock model!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     15,
				"completion_tokens": 5,
				"total_tokens":      20,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	_ = os.Setenv("TEST_KEY_ENV", "test-key")
	defer func() { _ = os.Unsetenv("TEST_KEY_ENV") }()

	client := NewClient(ts.URL, "TEST_KEY_ENV")
	resp, err := client.Chat(context.Background(), "mock-model", ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected Chat error: %v", err)
	}

	if resp.Content != "Hello from mock model!" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.Usage.TotalTokens != 20 {
		t.Errorf("expected total tokens 20, got %d", resp.Usage.TotalTokens)
	}
}

func TestClientToolCalling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "ping",
									"arguments": `{"message":"pong"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     30,
				"completion_tokens": 10,
				"total_tokens":      40,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "")
	resp, err := client.Chat(context.Background(), "mock-model", ChatRequest{
		Messages: []Message{{Role: "user", Content: "Call tool"}},
	})
	if err != nil {
		t.Fatalf("unexpected Chat error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "ping" {
		t.Errorf("expected tool ping, got %s", resp.ToolCalls[0].Function.Name)
	}
}

func TestClientRetry429(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit"}`))
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{"content": "recovered"},
				},
			},
			"usage": map[string]int{"total_tokens": 10},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "")
	resp, err := client.Chat(context.Background(), "mock", ChatRequest{})
	if err != nil {
		t.Fatalf("expected retry recovery, got error: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("expected recovered, got %s", resp.Content)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestClientNoRetry401(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "")
	_, err := client.Chat(context.Background(), "mock", ChatRequest{})
	if err == nil {
		t.Fatalf("expected 401 error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected only 1 attempt for 401, got %d", attempts)
	}
}
