package mockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Scenario defines how mock server should respond
type Scenario string

const (
	ScenarioHappy    Scenario = "happy"
	ScenarioRetry    Scenario = "retry"
	ScenarioBlocked  Scenario = "blocked"
	ScenarioBudget   Scenario = "budget"
	ScenarioConflict Scenario = "conflict"
)

// MockServer provides simulated OpenAI-compatible endpoints
type MockServer struct {
	server   *httptest.Server
	Scenario Scenario
	mu       sync.Mutex
	calls    map[string]int
}

// NewMockServer creates an OpenAI-compatible test server
func NewMockServer(s Scenario) *MockServer {
	ms := &MockServer{
		Scenario: s,
		calls:    make(map[string]int),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		messages, _ := req["messages"].([]any)
		lastContent := ""
		fullText := ""
		if len(messages) > 0 {
			lastMsg, _ := messages[len(messages)-1].(map[string]any)
			lastContent, _ = lastMsg["content"].(string)

			for _, m := range messages {
				mMap, _ := m.(map[string]any)
				c, _ := mMap["content"].(string)
				fullText += " " + c
			}
		}

		// 1. Master Planning Call
		if strings.Contains(fullText, "Generate the complete project DAG Plan JSON") {
			planJSON := `{
  "project": "demo_e2e",
  "tasks": [
    {
      "id": "T01",
      "title": "Create User Model",
      "type": "backend",
      "depends_on": [],
      "worker": "mock-worker",
      "max_retries": 2,
      "acceptance": ["echo user_model_ok"],
      "description": "Define User struct in user.go"
    },
    {
      "id": "T02",
      "title": "Create Auth API",
      "type": "backend",
      "depends_on": ["T01"],
      "worker": "mock-worker",
      "max_retries": 2,
      "acceptance": ["echo auth_api_ok"],
      "description": "Implement authentication endpoint"
    }
  ]
}`
			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": fmt.Sprintf("```json\n%s\n```", planJSON),
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     150,
					"completion_tokens": 80,
					"total_tokens":      230,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// 2. Worker Calls
		ms.mu.Lock()
		taskID := "T01"
		if strings.Contains(fullText, "T02") {
			taskID = "T02"
		}
		ms.calls[taskID]++
		turn := ms.calls[taskID]
		ms.mu.Unlock()

		// The worker prompt always has a HUMAN ANSWERS section; only the answer itself unblocks
		hasHumanAnswers := strings.Contains(fullText, "localhost:5432")

		// Reset turn counter if human unblocked task
		if hasHumanAnswers {
			turn = 1
		}

		// Skenario BLOCKED: jika ada kata kunci human answers, baru unblock
		if ms.Scenario == ScenarioBlocked && taskID == "T01" && !hasHumanAnswers {
			reportBlocked := "---\nstatus: BLOCKED\nattempts: 1\ncalls: 1\ntokens_in: 50\ntokens_out: 20\nfiles_changed: []\n---\n\n## Summary\nWaiting for human input.\n\n## Blockers\nNeed database host configuration."
			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{
									"id":   "call_block",
									"type": "function",
									"function": map[string]any{
										"name":      "write_file",
										"arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, taskID, reportBlocked),
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens": 40, "completion_tokens": 20, "total_tokens": 60,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 1: Worker writes implementation file and output report
		if turn == 1 {
			reportYAML := fmt.Sprintf("---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 100\ntokens_out: 40\nfiles_changed: [\"%s.txt\"]\n---\n\n## Summary\nImplemented task %s", strings.ToLower(taskID), taskID)

			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{
									"id":   "call_file",
									"type": "function",
									"function": map[string]any{
										"name":      "write_file",
										"arguments": fmt.Sprintf(`{"path":"%s.txt","content":"feature %s implementation\n"}`, strings.ToLower(taskID), taskID),
									},
								},
								{
									"id":   "call_report",
									"type": "function",
									"function": map[string]any{
										"name":      "write_file",
										"arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, taskID, reportYAML),
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 2: Completion speaking
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": fmt.Sprintf("Finished task %s completely.", taskID),
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 60, "completion_tokens": 10, "total_tokens": 70,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		_ = lastContent
	})

	ms.server = httptest.NewServer(handler)
	return ms
}

func (ms *MockServer) URL() string {
	return ms.server.URL
}

func (ms *MockServer) Close() {
	ms.server.Close()
}
