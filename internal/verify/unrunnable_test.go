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

// A program that is not installed fails the same way every time, so it is
// a question for the user, not another attempt: whether it is missing from
// PATH, or there and only saying it is unavailable, like Docker Desktop's
// docker in a WSL distro without WSL integration.
func TestMissingProgramIsUnrunnable(t *testing.T) {
	if why := MissingProgram("ringmaster-no-such-program --version"); !strings.Contains(why, "not installed") {
		t.Errorf("MissingProgram = %q", why)
	}
	if why := MissingProgram("go version"); why != "" {
		t.Errorf("go was reported missing: %q", why)
	}

	bin := t.TempDir()
	stub := "#!/bin/sh\necho \"The command 'docker' could not be found in this WSL 2 distro.\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	task := schemas.TaskSpec{ID: "T01", Acceptance: []string{"docker build -t shorty:test .", "ringmaster-no-such-program", "true"}}
	results, err := RunAcceptance(context.Background(), task, t.TempDir(), nil, nil, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %+v", results)
	}
	for _, r := range results[:2] {
		if !r.Unrunnable || !strings.Contains(r.StderrTail, "not installed") {
			t.Errorf("%s: want unrunnable, not installed: %+v", r.Command, r)
		}
	}
	if !results[2].Passed || results[2].Unrunnable {
		t.Errorf("true: %+v", results[2])
	}

	d := Decide(&schemas.WorkerReport{Status: schemas.ReportStatusCompleted}, results, 1, 2, false)
	if d.Type != DecisionNEED_REVIEW || len(d.Requests) != 1 || d.Requests[0].Options[0] != OptionChangeAcceptance {
		t.Errorf("decision = %+v, want a stop offering to change the acceptance commands", d)
	}
}
