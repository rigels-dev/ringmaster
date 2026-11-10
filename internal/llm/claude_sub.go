package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// Workers stream events so their progress is visible while they run; a
	// plain chat (e.g. the master planner) just waits for the single result.
	streaming := req.WorkDir != ""
	outputFormat := "--output-format=json"
	if streaming {
		outputFormat = "--output-format=stream-json"
	}
	args := []string{
		"-p", promptText,
		outputFormat,
		"--no-session-persistence",
	}
	if streaming {
		args = append(args, "--verbose") // required by the CLI for stream-json in print mode
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	// As a worker, Claude Code works in the task workspace with its own tools:
	// file edits are auto-accepted there, and Bash is limited to the sandbox allow list.
	if req.WorkDir != "" {
		allowed := []string{"Read", "Edit", "Write", "Glob", "Grep"}
		for _, command := range req.AllowedCommands {
			allowed = append(allowed, fmt.Sprintf("Bash(%s:*)", command))
		}
		args = append(args,
			"--permission-mode", "acceptEdits",
			"--allowedTools", strings.Join(allowed, ","),
		)
		systemPrompt += "\n\nYou are running as Claude Code inside the task workspace (the current directory). " +
			// The workspace path itself contains .ringmaster/ (worktrees live in the project's state dir),
			// so spell out how relative paths resolve or the report can land in the wrong .ringmaster/.
			"The workspace root is " + req.WorkDir + ". Every relative path in these instructions is relative to it, " +
			"so the report file goes to " + filepath.Join(req.WorkDir, ".ringmaster", "outputs") + "/<task id>.md. " +
			"Never read or write outside " + req.WorkDir + ". " +
			"Use your built-in tools (Read, Glob, Grep, Write, Edit, Bash) in place of read_file, list_dir, search, write_file and run_cmd. " +
			"Only these shell commands are permitted: " + strings.Join(req.AllowedCommands, ", ") + ". " +
			"Run one command per Bash call from the current directory, without cd, &&, pipes or redirects; create files with Write. " +
			"If you need any other command, request approval as described under WHEN YOU NEED THE USER instead of working around it. " +
			"Finish the whole task in this session, including writing the report file."
	}

	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, c.CLIPath, args...)
	cmd.Dir = req.WorkDir
	if req.WorkDir != "" && req.CommandEnv != nil {
		cmd.Env = claudeWorkerEnv(req.CommandEnv, os.Environ())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// In stream mode stdout is one JSON event per line; the final "result"
	// event has the same shape as the non-streaming output.
	var stream *streamEvents
	if streaming {
		stream = &streamEvents{workDir: req.WorkDir, onProgress: req.OnProgress}
		cmd.Stdout = &lineWriter{onLine: stream.handle}
	} else {
		cmd.Stdout = &stdout
	}

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" && stream != nil {
			errOutput = stream.lastText
		}
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("claude subscription CLI execution failed: %w (output: %s)", err, errOutput)
	}
	if stream != nil {
		if stream.result == nil {
			return nil, fmt.Errorf("claude subscription CLI ended without a result event (last message: %s)", stream.lastText)
		}
		stdout.Write(stream.result)
	}

	var rawResp struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			// Claude Code caches aggressively; input_tokens alone is only the uncached remainder
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
		StopReason   string  `json:"stop_reason"`
		TotalCostUSD float64 `json:"total_cost_usd"`
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
		Usage:        claudeUsage(rawResp.Usage.InputTokens, rawResp.Usage.CacheCreationInputTokens, rawResp.Usage.CacheReadInputTokens, rawResp.Usage.OutputTokens, rawResp.TotalCostUSD),
	}, nil
}

// claudeUsage counts every input token the model processed: the uncached
// input plus prompt-cache writes and reads, with the reads also kept apart.
func claudeUsage(input, cacheWrite, cacheRead, output int, costUSD float64) TokenUsage {
	prompt := input + cacheWrite + cacheRead
	return TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: output,
		TotalTokens:      prompt + output,
		CachedTokens:     cacheRead,
		CostUSD:          costUSD,
	}
}

func (c *ClaudeSubscriptionClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-opus-5-5", Desc: "Claude Opus 5.5 (Deep Architecture & Planning)"},
		{ID: "claude-sonnet-5-5", Desc: "Claude Sonnet 5.5 (Balanced)"},
		{ID: "claude-haiku-5-5", Desc: "Claude Haiku 5.5 (Fast Worker)"},
		{ID: "claude-fable-5-1", Desc: "Claude Fable 5.1"},
	}, nil
}

// lineWriter calls onLine for every complete line written to it.
type lineWriter struct {
	buf    []byte
	onLine func([]byte)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimSpace(w.buf[:i])
		if len(line) > 0 {
			w.onLine(line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// streamEvents interprets Claude Code's stream-json events: assistant tool
// calls and messages become progress events, the result event is kept.
type streamEvents struct {
	workDir    string
	onProgress func(ProgressEvent)
	result     []byte
	lastText   string
}

func (s *streamEvents) handle(line []byte) {
	var ev struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}

	switch ev.Type {
	case "result":
		s.result = append([]byte(nil), line...)
	case "assistant":
		for _, block := range ev.Message.Content {
			switch block.Type {
			case "tool_use":
				s.emit(ProgressEvent{Kind: "tool", Tool: block.Name, Detail: s.summarizeInput(block.Input), Input: block.Input})
			case "text":
				if text := strings.TrimSpace(block.Text); text != "" {
					s.lastText = text
					s.emit(ProgressEvent{Kind: "text", Detail: text})
				}
			}
		}
	}
}

func (s *streamEvents) emit(ev ProgressEvent) {
	if s.onProgress != nil {
		s.onProgress(ev)
	}
}

// summarizeInput picks the one field of a tool input worth showing in a log
// line, with workspace paths made relative.
func (s *streamEvents) summarizeInput(input json.RawMessage) string {
	var fields map[string]any
	if json.Unmarshal(input, &fields) != nil {
		return ""
	}
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "description"} {
		v, ok := fields[key].(string)
		if !ok || v == "" {
			continue
		}
		if s.workDir != "" && filepath.IsAbs(v) {
			if rel, err := filepath.Rel(s.workDir, v); err == nil && !strings.HasPrefix(rel, "..") {
				v = rel
			}
		}
		return v
	}
	return ""
}

// claudeCLIEnvPrefixes are the variables Claude Code reads for its own login
// and settings (CLAUDE_CONFIG_DIR, CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_BASE_URL, ...).
var claudeCLIEnvPrefixes = []string{"CLAUDE_", "ANTHROPIC_"}

// claudeWorkerEnv is the environment for a Claude Code worker: the filtered
// command environment, plus the CLI's own variables from the full one. The CLI
// runs the worker's shell commands itself and hands them everything it was
// started with, so this is the only place to keep the rest of the caller's
// environment (cloud keys, database URLs) away from those commands.
func claudeWorkerEnv(commandEnv, full []string) []string {
	env := append([]string(nil), commandEnv...)
	have := make(map[string]bool, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		have[name] = true
	}
	for _, kv := range full {
		name, _, _ := strings.Cut(kv, "=")
		if have[name] {
			continue
		}
		for _, prefix := range claudeCLIEnvPrefixes {
			if strings.HasPrefix(name, prefix) {
				env = append(env, kv)
				break
			}
		}
	}
	return env
}
