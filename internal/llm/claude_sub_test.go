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
