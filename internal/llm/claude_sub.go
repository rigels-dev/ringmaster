package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ClaudeSubscriptionClient struct {
	CLIPath string
}

func NewClaudeSubscriptionClient() *ClaudeSubscriptionClient {
	cliPath, _ := exec.LookPath("claude")
	return &ClaudeSubscriptionClient{
		CLIPath: cliPath,
	}
}

func (c *ClaudeSubscriptionClient) Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error) {
	if c.CLIPath == "" {
		cliPath, err := exec.LookPath("claude")
		if err != nil {
			return nil, fmt.Errorf("claude CLI not found in PATH: please ensure Claude Code is installed ('npm install -g @anthropic-ai/claude-code') and authenticated with your Claude subscription ('claude login')")
		}
		c.CLIPath = cliPath
	}

	var systemPrompt string
	var conversation strings.Builder

	for _, m := range req.Messages {
		if strings.ToLower(m.Role) == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += m.Content
			continue
		}

		if m.ToolCallID != "" {
			conversation.WriteString(fmt.Sprintf("\n[Tool Output %s]:\n%s\n", m.ToolCallID, m.Content))
			continue
		}

		if len(m.ToolCalls) > 0 {
			conversation.WriteString(fmt.Sprintf("\n[Assistant Tool Calls]:\n"))
			for _, tc := range m.ToolCalls {
				conversation.WriteString(fmt.Sprintf("- %s: %s\n", tc.Function.Name, tc.Function.Arguments))
			}
			if m.Content != "" {
				conversation.WriteString(m.Content + "\n")
			}
			continue
		}

		roleLabel := strings.ToUpper(m.Role)
		if roleLabel == "USER" {
			conversation.WriteString(fmt.Sprintf("\nUser:\n%s\n", m.Content))
		} else {
			conversation.WriteString(fmt.Sprintf("\nAssistant:\n%s\n", m.Content))
		}
	}

	promptText := strings.TrimSpace(conversation.String())
	if promptText == "" {
		promptText = "Hello"
	}

	args := []string{
		"-p", promptText,
		"--output-format=json",
		"--no-session-persistence",
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, c.CLIPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("claude subscription CLI execution failed: %w (output: %s)", err, errOutput)
	}

	var rawResp struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		StopReason string `json:"stop_reason"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &rawResp); err != nil {
		// Fallback: if not valid JSON, treat raw stdout as text
		return &ChatResponse{
			Content:      strings.TrimSpace(stdout.String()),
			FinishReason: "end_turn",
		}, nil
	}

	if rawResp.IsError {
		return nil, fmt.Errorf("claude subscription error: %s", rawResp.Result)
	}

	finishReason := rawResp.StopReason
	if finishReason == "" {
		finishReason = "end_turn"
	}

	return &ChatResponse{
		Content:      rawResp.Result,
		FinishReason: finishReason,
		Usage: TokenUsage{
			PromptTokens:     rawResp.Usage.InputTokens,
			CompletionTokens: rawResp.Usage.OutputTokens,
			TotalTokens:      rawResp.Usage.InputTokens + rawResp.Usage.OutputTokens,
		},
	}, nil
}

func (c *ClaudeSubscriptionClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "sonnet", Desc: "Claude Sonnet (Master Planner / Lead Architecture)"},
		{ID: "haiku", Desc: "Claude Haiku (Fast Worker Coder)"},
		{ID: "opus", Desc: "Claude Opus (Deep Reasoning & Hard Problems)"},
	}, nil
}
