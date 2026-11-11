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
	// ResolvePath returns resolved paths, and the temp dir itself may sit
	// under a symlink (on macOS, /var is one)
	realTempDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resolved, realTempDir+string(filepath.Separator)) {
		t.Errorf("expected path to stay inside %s, got: %s", realTempDir, resolved)
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
										"name":      "write_file",
										"arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/T01.md","content":%q}`, reportYAML),
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
						"content": "All done! Final report written to .ringmaster/outputs/T01.md",
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
		Acceptance:  []string{".ringmaster/outputs/T01.md exists"},
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

// A worker is told about the skills in its workspace, by name and description
// only, and can read them with its ordinary file tool. Without skills the
// prompt does not mention them.
func TestRunnerListsWorkspaceSkills(t *testing.T) {
	t.Setenv("DUMMY_KEY", "mock-token-value")

	systemPromptOf := func(worktree string) string {
		var system string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			for _, m := range req.Messages {
				if m.Role == "system" {
					system = m.Content
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "done"}}},
				"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
			})
		}))
		defer ts.Close()

		task := schemas.TaskSpec{ID: "T01", Title: "t", Description: "d", Acceptance: []string{"echo ok"}}
		agent := config.AgentConfig{Model: "mock-model", Role: "worker", ToolCalling: true}
		_, err := RunAttempt(context.Background(), task, worktree, agent, llm.NewProvider("openai", ts.URL, "DUMMY_KEY"),
			config.BudgetConfig{MaxTokensPerTask: 10000}, config.SandboxConfig{Allow: []string{"echo"}}, "brief", 1)
		if err != nil {
			t.Fatalf("RunAttempt failed: %v", err)
		}
		return system
	}

	plain := t.TempDir()
	if prompt := systemPromptOf(plain); strings.Contains(prompt, "SKILLS") {
		t.Errorf("prompt mentions skills though none are installed")
	}

	withSkill := t.TempDir()
	skillDir := filepath.Join(withSkill, ".ringmaster", "skills", "go-style")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "---\ndescription: House rules for Go code\n---\n\nALWAYS-WRAP-ERRORS\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	prompt := systemPromptOf(withSkill)
	if !strings.Contains(prompt, "- go-style (.ringmaster/skills/go-style/SKILL.md): House rules for Go code") {
		t.Errorf("prompt does not list the skill:\n%s", prompt)
	}
	if strings.Contains(prompt, "ALWAYS-WRAP-ERRORS") {
		t.Errorf("the skill's body was put in the prompt; only its description belongs there")
	}
	if strings.Index(prompt, "# SKILLS:") > strings.Index(prompt, "# ACCEPTANCE CRITERIA:") {
		t.Errorf("skills section should come before the acceptance criteria")
	}

	// The path in the prompt is one the worker's own read tool accepts
	out := NewToolSet(withSkill, nil).Execute(context.Background(), "read_file", map[string]any{"path": ".ringmaster/skills/go-style/SKILL.md"})
	if !strings.Contains(out, "ALWAYS-WRAP-ERRORS") {
		t.Errorf("read_file could not read the skill: %s", out)
	}
}
