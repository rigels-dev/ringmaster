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

	// Normalize root. Resolve its symlinks too, so it compares against resolved
	// targets (on macOS /tmp is a symlink to /private/tmp)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", err)
	}
	if realRoot, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		cleanRoot = realRoot
	}

	fullPath := filepath.Join(cleanRoot, cleanRel)

	// Guard against path traversal like ../../etc/passwd
	if !isWithin(cleanRoot, fullPath) {
		return "", fmt.Errorf("path escapes sandbox boundary: %s", rel)
	}

	// Resolve the deepest part of the path that exists and check where it really
	// points. The rest (not created yet, e.g. for write_file) is appended as is.
	existing, rest := fullPath, ""
	for {
		realPath, err := filepath.EvalSymlinks(existing)
		if err == nil {
			if !isWithin(cleanRoot, realPath) {
				return "", fmt.Errorf("symlink target escapes sandbox boundary: %s", rel)
			}
			return filepath.Join(realPath, rest), nil
		}
		// Present but unresolvable: a dangling symlink, which a write would follow
		if _, lerr := os.Lstat(existing); lerr == nil {
			return "", fmt.Errorf("unresolvable symlink in path: %s", rel)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("path escapes sandbox boundary: %s", rel)
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}

// isWithin reports whether path is root itself or lies below it. It compares
// whole path components, so /work/T01 does not contain /work/T010.
func isWithin(root, path string) bool {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator))
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
