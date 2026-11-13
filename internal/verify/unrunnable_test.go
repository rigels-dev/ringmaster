package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ringmaster/internal/schemas"
)

func TestUnrunnableReason(t *testing.T) {
	runnable := []string{
		"pytest -q",
		"go test ./...",
		"npm test",
		"python -m wordstat README.md --top 1",
		"node --test tests/",
		`python -c "import wordstat"`,
		`python -c "import pathlib; assert pathlib.Path('README.md').exists()"`,
		"grep 'python -m wordstat' README.md",
		`grep -q "a | b" notes.txt`,
		"python -c pass",
	}
	for _, cmd := range runnable {
		if why := UnrunnableReason(cmd); why != "" {
			t.Errorf("UnrunnableReason(%q) = %q, want none", cmd, why)
		}
	}

	unrunnable := []string{
		"grep -q json README.md && echo ok",
		"cat README.md | grep json",
		"test -f README.md; echo done",
		"echo $(date)",
		`echo "$(date)"`,
		"go test ./... > out.txt",
		"echo `date`",
		`python -c "import wordstat`,
		// The case from a real run: an inline program without its quotes
		"python -c import wordstat",
		"python3 -c from wordstat import cli",
		"node -e require ./index.js",
		"",
	}
	for _, cmd := range unrunnable {
		if why := UnrunnableReason(cmd); why == "" {
			t.Errorf("UnrunnableReason(%q) = none, want a reason", cmd)
		}
	}

	why := UnrunnableReason("python -c import wordstat")
	if !strings.Contains(why, `python -c "import wordstat"`) {
		t.Errorf("the reason does not show the corrected command: %s", why)
	}
}

// A quoted argument reaches the program as one argument. Before, the command
// was split on every space, so the program after -c was cut at its first word.
func TestAcceptanceKeepsQuotedArgumentsTogether(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	task := schemas.TaskSpec{ID: "T01", Acceptance: []string{
		`sh -c "test -f marker.txt"`,
		`sh -c "test -f missing.txt"`,
		`sh -c 'exit 7'`,
	}}

	checks, err := RunAcceptance(context.Background(), task, dir, []string{"sh"}, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 3 {
		t.Fatalf("got %d results, want 3", len(checks))
	}
	if !checks[0].Passed {
		t.Errorf("check 1 should pass: exit %d, %s", checks[0].ExitCode, checks[0].StderrTail)
	}
	if checks[1].Passed || checks[1].Unrunnable {
		t.Errorf("check 2 should fail as an ordinary failed check: %+v", checks[1])
	}
	if checks[2].ExitCode != 7 {
		t.Errorf("check 3 exit code = %d, want 7 (the quoted program ran as written)", checks[2].ExitCode)
	}
}
