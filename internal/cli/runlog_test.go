package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func readRunLog(t *testing.T, projectDir string) []RunLogEntry {
	t.Helper()
	f, err := os.Open(filepath.Join(projectDir, ".ringmaster", "logs", RunLogName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var entries []RunLogEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e RunLogEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad log line %q: %v", sc.Text(), err)
		}
		entries = append(entries, e)
	}
	return entries
}

// Output of every run is appended to run.jsonl with markers, so earlier runs
// (and their finished tasks) are not lost when a new run starts.
func TestRunLogKeepsEveryRun(t *testing.T) {
	project := t.TempDir()

	for run := 1; run <= 2; run++ {
		stop, err := startRunLog(project, fmt.Sprintf("Run started #%d", run))
		if err != nil {
			t.Fatalf("startRunLog: %v", err)
		}
		fmt.Printf("[T0%d] step 1: exec `go test ./...`\n", run)
		fmt.Fprintln(os.Stderr, "something on stderr")
		stop(fmt.Sprintf("Run ended #%d", run))
	}

	entries := readRunLog(t, project)
	if len(entries) != 8 {
		t.Fatalf("got %d entries, want 8: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Time == "" {
			t.Errorf("entry without timestamp: %+v", e)
		}
	}

	// Each run is framed by its markers; stdout and stderr are separate
	// streams, so only their order relative to each other may vary.
	for run := 1; run <= 2; run++ {
		block := entries[(run-1)*4 : run*4]
		if block[0].Line != fmt.Sprintf("[Run] Run started #%d", run) || block[3].Line != fmt.Sprintf("[Run] Run ended #%d", run) {
			t.Fatalf("run %d is not framed by its markers: %+v", run, block)
		}
		seen := map[string]string{}
		for _, e := range block[1:3] {
			seen[e.Line] = e.Stream
		}
		if stream, ok := seen[fmt.Sprintf("[T0%d] step 1: exec `go test ./...`", run)]; !ok || stream != "" {
			t.Errorf("run %d: stdout line missing or mislabeled: %+v", run, block)
		}
		if stream, ok := seen["something on stderr"]; !ok || stream != "err" {
			t.Errorf("run %d: stderr line missing or not marked: %+v", run, block)
		}
	}
}
