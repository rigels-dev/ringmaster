package worker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ringmaster/internal/llm"
)

const (
	maxOutputBytes = 20 * 1024 // 20 KB buffer truncation
	maxCommandWait = 120 * time.Second
)

// ToolSet defines the tools available to worker agents.
type ToolSet struct {
	worktreeRoot string
	allowlist    []string

	// PassEnv lists extra environment variables run_cmd hands to commands,
	// on top of the defaults (see CommandEnv)
	PassEnv []string
}

// NewToolSet creates a ToolSet scoped to a specific worktree directory.
func NewToolSet(worktreeRoot string, allowlist []string) *ToolSet {
	return &ToolSet{
		worktreeRoot: worktreeRoot,
		allowlist:    allowlist,
	}
}

// Definitions returns standard OpenAI-compatible tool definitions for LLMs.
func (ts *ToolSet) Definitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "read_file",
				Description: "Read content of a file within the worktree.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Relative path to file"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "write_file",
				Description: "Write or overwrite content of a file within the worktree.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string", "description": "Relative path to file"},
						"content": map[string]any{"type": "string", "description": "Text content to write"},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "list_dir",
				Description: "List entries in a directory relative to worktree root.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Relative directory path (or empty/'.' for root)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "search",
				Description: "Search files using a regex pattern within the worktree.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{"type": "string", "description": "Regular expression pattern to find"},
						"path":    map[string]any{"type": "string", "description": "Subdirectory to search in (optional)"},
					},
					"required": []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "run_cmd",
				Description: "Execute a command permitted by sandbox allowlist.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"cmd": map[string]any{"type": "string", "description": "Command string to execute"},
					},
					"required": []string{"cmd"},
				},
			},
		},
	}
}

// Execute executes a tool call by name with parsed arguments.
func (ts *ToolSet) Execute(ctx context.Context, name string, args map[string]any) string {
	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		return ts.readFile(path)
	case "write_file":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		return ts.writeFile(path, content)
	case "list_dir":
		path, _ := args["path"].(string)
		return ts.listDir(path)
	case "search":
		pattern, _ := args["pattern"].(string)
		subpath, _ := args["path"].(string)
		return ts.search(pattern, subpath)
	case "run_cmd":
		cmd, _ := args["cmd"].(string)
		return ts.runCmd(ctx, cmd)
	default:
		return fmt.Sprintf("Error: unknown tool '%s'", name)
	}
}

func (ts *ToolSet) readFile(relPath string) string {
	target, err := ResolvePath(ts.worktreeRoot, relPath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	if len(data) > maxOutputBytes {
		return string(data[:maxOutputBytes]) + fmt.Sprintf("\n... [truncated at %d bytes]", maxOutputBytes)
	}
	return string(data)
}

func (ts *ToolSet) writeFile(relPath, content string) string {
	target, err := ResolvePath(ts.worktreeRoot, relPath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Error creating parent directory: %v", err)
	}

	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), relPath)
}

func (ts *ToolSet) listDir(relPath string) string {
	if relPath == "" {
		relPath = "."
	}
	target, err := ResolvePath(ts.worktreeRoot, relPath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return fmt.Sprintf("Error listing directory: %v", err)
	}

	var sb strings.Builder
	for _, e := range entries {
		kind := "FILE"
		if e.IsDir() {
			kind = "DIR "
		}
		sb.WriteString(fmt.Sprintf("%s  %s\n", kind, e.Name()))
	}

	res := sb.String()
	if res == "" {
		return "(directory is empty)"
	}
	return res
}

func (ts *ToolSet) search(pattern, relPath string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("Error: invalid regex pattern: %v", err)
	}

	// Work from the resolved root so relative paths stay inside it even when
	// the worktree is reached through a symlink
	root, err := ResolvePath(ts.worktreeRoot, ".")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	searchRoot := root
	if relPath != "" && relPath != "." {
		target, err := ResolvePath(ts.worktreeRoot, relPath)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		searchRoot = target
	}

	var matches []string
	_ = filepath.Walk(searchRoot, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}

		// Skip git internals
		if strings.Contains(p, "/.git/") {
			return nil
		}

		rel, _ := filepath.Rel(root, p)

		// Walk does not follow symlinks but reading one would, so a link is
		// read only if it resolves to a regular file inside the worktree.
		// Anything else that is not a regular file (FIFO, socket, device) is skipped.
		if !info.Mode().IsRegular() {
			if info.Mode()&os.ModeSymlink == 0 {
				return nil
			}
			resolved, err := ResolvePath(root, rel)
			if err != nil {
				return nil
			}
			if st, err := os.Stat(resolved); err != nil || !st.Mode().IsRegular() {
				return nil
			}
			p = resolved
		}

		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, lineNum+1, strings.TrimSpace(line)))
				if len(matches) >= 100 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if len(matches) == 0 {
		return "No matches found."
	}
	return strings.Join(matches, "\n")
}

func (ts *ToolSet) runCmd(ctx context.Context, cmdStr string) string {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return "Error: empty command string"
	}

	if !CheckCommand(cmdStr, ts.allowlist) {
		return fmt.Sprintf("Error: command '%s' is not in sandbox allowlist", cmdStr)
	}

	fields := strings.Fields(cmdStr)
	c := fields[0]
	args := fields[1:]

	timeoutCtx, cancel := context.WithTimeout(ctx, maxCommandWait)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, c, args...)
	cmd.Dir = ts.worktreeRoot
	cmd.Env = CommandEnv(ts.PassEnv)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	output := outBuf.Bytes()

	if len(output) > maxOutputBytes {
		output = append(output[:maxOutputBytes], []byte(fmt.Sprintf("\n... [truncated at %d bytes]", maxOutputBytes))...)
	}

	if err != nil {
		return fmt.Sprintf("Command failed with error: %v\nOutput:\n%s", err, string(output))
	}

	if len(output) == 0 {
		return "(command completed with no output)"
	}
	return string(output)
}
