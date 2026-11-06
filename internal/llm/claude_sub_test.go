package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeSubscriptionMock(t *testing.T) {
	tempDir := t.TempDir()
	mockClaudePath := filepath.Join(tempDir, "claude")

	script := `#!/bin/sh
cat << 'EOF'
{
  "is_error": false,
  "result": "Hello from mock subscription!",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5
  },
  "stop_reason": "end_turn"
}
EOF
`
	if err := os.WriteFile(mockClaudePath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	client := &ClaudeSubscriptionClient{
		CLIPath: mockClaudePath,
	}

	resp, err := client.Chat(context.Background(), "sonnet", ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from mock subscription!" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
}

// As a worker (WorkDir set) the CLI streams events; each tool call and message
// must reach OnProgress live, and the result event still forms the response.
func TestClaudeSubscriptionStreamsWorkerProgress(t *testing.T) {
	tempDir := t.TempDir()
	workDir := t.TempDir()
	mockClaudePath := filepath.Join(tempDir, "claude")

	script := `#!/bin/sh
case "$*" in *stream-json*--verbose*) ;; *) echo "expected stream-json with --verbose: $*" >&2; exit 2;; esac
echo '{"type":"system","subtype":"init"}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"` + workDir + `/go.mod"}}]}}'
echo '{"type":"user","message":{"content":[{"type":"tool_result","content":"module x"}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"All tests pass."}]}}'
echo '{"type":"result","subtype":"success","is_error":false,"result":"Done.","stop_reason":"end_turn","total_cost_usd":0.0125,"usage":{"input_tokens":7,"cache_creation_input_tokens":100,"cache_read_input_tokens":900,"output_tokens":3}}'
`
	if err := os.WriteFile(mockClaudePath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	var events []ProgressEvent
	client := &ClaudeSubscriptionClient{CLIPath: mockClaudePath}
	resp, err := client.Chat(context.Background(), "haiku", ChatRequest{
		Messages:   []Message{{Role: "user", Content: "work"}},
		WorkDir:    workDir,
		OnProgress: func(ev ProgressEvent) { events = append(events, ev) },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Done." || resp.Usage.CompletionTokens != 3 {
		t.Errorf("unexpected response: %+v", resp)
	}
	// Input counts uncached input plus cache writes and reads
	if resp.Usage.PromptTokens != 1007 || resp.Usage.CachedTokens != 900 || resp.Usage.TotalTokens != 1010 {
		t.Errorf("cache tokens not counted: %+v", resp.Usage)
	}
	if resp.Usage.CostUSD != 0.0125 {
		t.Errorf("cost not kept: %+v", resp.Usage)
	}

	want := []ProgressEvent{
		{Kind: "tool", Tool: "Read", Detail: "go.mod"},
		{Kind: "tool", Tool: "Bash", Detail: "go test ./..."},
		{Kind: "text", Detail: "All tests pass."},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d progress events, want %d: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		if events[i].Kind != w.Kind || events[i].Tool != w.Tool || events[i].Detail != w.Detail {
			t.Errorf("event %d = %+v, want %+v", i, events[i], w)
		}
	}
}
