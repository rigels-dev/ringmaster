package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath ensures rel path stays securely inside root, rejecting path traversal (..)
// absolute path escapes, and symlink breakout.
func ResolvePath(root, rel string) (string, error) {
	cleanRel := filepath.Clean(rel)
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}

	// Normalize root
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", err)
	}

	fullPath := filepath.Join(cleanRoot, cleanRel)

	// Guard against path traversal like ../../etc/passwd
	if !strings.HasPrefix(fullPath, cleanRoot) {
		return "", fmt.Errorf("path escapes sandbox boundary: %s", rel)
	}

	// Check if target or intermediate exists and resolves outside boundary via symlink
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err == nil {
		if !strings.HasPrefix(realPath, cleanRoot) {
			return "", fmt.Errorf("symlink target escapes sandbox boundary: %s", rel)
		}
		return realPath, nil
	}

	// If file doesn't exist yet (e.g. for write_file), check deepest existing parent directory
	parent := filepath.Dir(fullPath)
	realParent, err := filepath.EvalSymlinks(parent)
	if err == nil {
		if !strings.HasPrefix(realParent, cleanRoot) {
			return "", fmt.Errorf("parent directory escapes sandbox boundary: %s", rel)
		}
	}

	return fullPath, nil
}

// CheckCommand checks if cmd string matches the allowlist by token prefix (not substring).
// e.g. "go test -v" matches allowlist "go test", but "rm -rf" does not.
func CheckCommand(cmd string, allowlist []string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return false
	}

	for _, allowed := range allowlist {
		allowFields := strings.Fields(strings.TrimSpace(allowed))
		if len(allowFields) == 0 {
			continue
		}

		if len(fields) >= len(allowFields) {
			match := true
			for i := range allowFields {
				if fields[i] != allowFields[i] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// ScrubbedEnv filters out environment variables containing sensitive keywords
// (KEY, TOKEN, SECRET, PASSWORD) to prevent credential leaks inside worker commands.
func ScrubbedEnv() []string {
	sensitive := []string{"KEY", "TOKEN", "SECRET", "PASSWORD"}
	var scrubbed []string

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 0 {
			continue
		}
		keyUpper := strings.ToUpper(parts[0])
		isSensitive := false
		for _, s := range sensitive {
			if strings.Contains(keyUpper, s) {
				isSensitive = true
				break
			}
		}
		if !isSensitive {
			scrubbed = append(scrubbed, env)
		}
	}

	return scrubbed
}
