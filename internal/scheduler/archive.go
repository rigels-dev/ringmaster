package scheduler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// archiveTaskArtifacts copies a task's report and attempt transcripts from its
// worktree into the project's .ringmaster/, so they survive the worktree being
// removed after a merge. Later copies overwrite earlier ones.
func archiveTaskArtifacts(projectRoot, worktree, taskID string) error {
	src := filepath.Join(worktree, ".ringmaster")
	dst := filepath.Join(projectRoot, ".ringmaster")

	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	report := filepath.Join(src, "outputs", taskID+".md")
	if _, err := os.Stat(report); err == nil {
		keep(copyFile(report, filepath.Join(dst, "outputs", taskID+".md")))
	}

	entries, _ := os.ReadDir(filepath.Join(src, "logs"))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, taskID+"-attempt") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		keep(copyFile(filepath.Join(src, "logs", name), filepath.Join(dst, "logs", name)))
	}
	return firstErr
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return err
	}
	tmp := to + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("copy %s: %w", from, err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, to)
}
