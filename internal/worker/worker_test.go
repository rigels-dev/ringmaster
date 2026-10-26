package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
)

func TestSandboxResolvePath(t *testing.T) {
	tempDir := t.TempDir()

	// Safe relative file
	resolved, err := ResolvePath(tempDir, "src/main.go")
	if err != nil {
		t.Fatalf("expected valid path, got error: %v", err)
	}
	if !strings.HasPrefix(resolved, tempDir) {
		t.Errorf("expected path to stay inside tempDir, got: %s", resolved)
	}

	// Path traversal attempt
	_, err = ResolvePath(tempDir, "../../etc/passwd")
	if err == nil {
		t.Errorf("expected path traversal to be rejected, but succeeded")
	}

	// Absolute path attempt
	_, err = ResolvePath(tempDir, "/etc/shadow")
	if err == nil {
		t.Errorf("expected absolute path to be rejected, but succeeded")
	}
}

func TestSandboxCheckCommand(t *testing.T) {
	allowlist := []string{"go test", "pytest", "npm test", "git status"}

	if !CheckCommand("go test -v ./...", allowlist) {
		t.Errorf("expected 'go test -v ./...' to be allowed")
	}
	if !CheckCommand("pytest tests/test_api.py", allowlist) {
		t.Errorf("expected 'pytest tests/test_api.py' to be allowed")
	}
	if CheckCommand("rm -rf /", allowlist) {
		t.Errorf("expected 'rm -rf /' to be rejected")
	}
	if CheckCommand("sudo reboot", allowlist) {
		t.Errorf("expected 'sudo reboot' to be rejected")
	}
	if CheckCommand("go build && rm -rf .", allowlist) {
		// prefix match check: fields[0]=go, fields[1]=build -> not in allowlist
		t.Errorf("expected 'go build' not in allowlist to be rejected")
	}
}

func TestSandboxScrubbedEnv(t *testing.T) {
	t.Setenv("SECRET_API_KEY", "sensitive_123")
	t.Setenv("AUTH_TOKEN", "bearer_token")
	t.Setenv("NORMAL_ENV", "safe_val")

	scrubbed := ScrubbedEnv()
	for _, env := range scrubbed {
		if strings.Contains(env, "SECRET_API_KEY") || strings.Contains(env, "AUTH_TOKEN") {
			t.Errorf("sensitive env was not scrubbed: %s", env)
		}
	}
}

func TestToolSetOperations(t *testing.T) {
	tempDir := t.TempDir()
	ts := NewToolSet(tempDir, []string{"echo"})

	// write_file
	writeRes := ts.Execute(context.Background(), "write_file", map[string]any{
		"path":    "hello.txt",
		"content": "Hello Ringmaster!",
	})
	if !strings.Contains(writeRes, "Successfully wrote") {
		t.Fatalf("write_file failed: %s", writeRes)
	}

	// read_file
	readRes := ts.Execute(context.Background(), "read_file", map[string]any{
		"path": "hello.txt",
	})
	if readRes != "Hello Ringmaster!" {
		t.Fatalf("expected 'Hello Ringmaster!', got '%s'", readRes)
	}

	// list_dir
	listRes := ts.Execute(context.Background(), "list_dir", map[string]any{
		"path": ".",
	})
	if !strings.Contains(listRes, "hello.txt") {
		t.Fatalf("list_dir missing hello.txt: %s", listRes)
	}

	// search
	searchRes := ts.Execute(context.Background(), "search", map[string]any{
		"pattern": "Ringmaster",
	})
	if !strings.Contains(searchRes, "hello.txt:1: Hello Ringmaster!") {
		t.Fatalf("search missing match: %s", searchRes)
	}

	// run_cmd allowed
	cmdRes := ts.Execute(context.Background(), "run_cmd", map[string]any{
		"cmd": "echo test_ok",
	})
	if !strings.Contains(cmdRes, "test_ok") {
		t.Fatalf("run_cmd failed: %s", cmdRes)
	}

	// run_cmd disallowed
	disallowedRes := ts.Execute(context.Background(), "run_cmd", map[string]any{
		"cmd": "rm -rf hello.txt",
	})
	if !strings.Contains(disallowedRes, "not in sandbox allowlist") {
		t.Fatalf("expected rejection for disallowed command, got: %s", disallowedRes)
	}
}

func TestRunnerAttemptExecution(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DUMMY_KEY", "mock-token-value")

	turnCount := 0
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		w.Header().Set("Content-Type", "application/json")

		if turnCount == 1 {
			// First turn: invoke tool write_file for task output
			reportYAML := "---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 100\ntokens_out: 50\nfiles_changed: []\n---\n\n## Summary\nTask completed successfully."
			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name": "write_file",
										"arguments": fmt.Sprintf(`{"path":"outputs/T01.md","content":%q}`, reportYAML),
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     100,
					"completion_tokens": 50,
					"total_tokens":      150,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Second turn: complete without further tools
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "All done! Final report written to outputs/T01.md",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     120,
				"completion_tokens": 20,
				"total_tokens":      140,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tsMock.Close()

	provider := llm.NewProvider("openai", tsMock.URL, "DUMMY_KEY")

	task := schemas.TaskSpec{
		ID:          "T01",
		Title:       "Implement Auth Module",
		Type:        schemas.TaskTypeBackend,
		Description: "Create authentication logic",
		Acceptance:  []string{"outputs/T01.md exists"},
	}

	agent := config.AgentConfig{
		Provider:    "test_prov",
		Model:       "mock-model",
		Role:        "worker",
		ToolCalling: true,
	}

	budget := config.BudgetConfig{
		MaxTokensPerTask: 10000,
	}

	sandboxCfg := config.SandboxConfig{
		Allow: []string{"echo"},
	}

	res, err := RunAttempt(context.Background(), task, tempDir, agent, provider, budget, sandboxCfg, "Project Brief Demo", 1)
	if err != nil {
		t.Fatalf("RunAttempt failed: %v", err)
	}

	if res.Status != schemas.ReportStatusCompleted {
		t.Errorf("expected status COMPLETED, got: %s", res.Status)
	}
	if res.Reason != "COMPLETED" {
		t.Errorf("expected reason COMPLETED, got: %s", res.Reason)
	}
	if res.Turns != 2 {
		t.Errorf("expected 2 turns, got: %d", res.Turns)
	}
	if res.Usage.TokensTotal() != 290 {
		t.Errorf("expected 290 total tokens, got: %d", res.Usage.TokensTotal())
	}

	// Verify transcript log exists
	logContent, err := os.ReadFile(filepath.Join(tempDir, ".ringmaster", "logs", "T01-attempt1.jsonl"))
	if err != nil {
		t.Fatalf("failed reading transcript log: %v", err)
	}
	if !strings.Contains(string(logContent), "call_1") {
		t.Errorf("transcript log missing call_1 tool execution")
	}
}

func TestRunnerBudgetLimitEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DUMMY_KEY", "mock-token-value")

	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "list_dir",
									"arguments": `{"path":"."}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1000,
				"completion_tokens": 500,
				"total_tokens":      1500,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tsMock.Close()

	provider := llm.NewProvider("openai", tsMock.URL, "DUMMY_KEY")

	task := schemas.TaskSpec{
		ID:         "T02",
		Title:      "Heavy Task",
		Type:       schemas.TaskTypeBackend,
		Acceptance: []string{"check"},
	}
	agent := config.AgentConfig{Model: "mock", Role: "worker"}
	budget := config.BudgetConfig{MaxTokensPerTask: 1000} // limit is 1000
	sandboxCfg := config.SandboxConfig{}

	res, err := RunAttempt(context.Background(), task, tempDir, agent, provider, budget, sandboxCfg, "Brief", 1)
	if err != nil {
		t.Fatalf("RunAttempt error: %v", err)
	}

	if res.Reason != "BUDGET" {
		t.Errorf("expected reason BUDGET, got: %s", res.Reason)
	}
	if res.Status != schemas.ReportStatusNeedsReview {
		t.Errorf("expected status NEEDS_REVIEW on budget break, got: %s", res.Status)
	}
}
