package worker

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"go test ./...", []string{"go", "test", "./..."}},
		{"  pytest   -q  ", []string{"pytest", "-q"}},
		{`python -c "import wordstat"`, []string{"python", "-c", "import wordstat"}},
		{`python -c 'import a; print(a.x)'`, []string{"python", "-c", "import a; print(a.x)"}},
		{`python -c "assert '--json' in t"`, []string{"python", "-c", "assert '--json' in t"}},
		{`grep "a | b > c" file.txt`, []string{"grep", "a | b > c", "file.txt"}},
		{`echo "say \"hi\""`, []string{"echo", `say "hi"`}},
		{`echo a\ b`, []string{"echo", "a b"}},
		{`echo "" x`, []string{"echo", "", "x"}},
		{`echo pre"mid dle"post`, []string{"echo", "premid dlepost"}},
		{`echo '$HOME' "$HOME"`, []string{"echo", "$HOME", "$HOME"}},
	}
	for _, c := range cases {
		got, err := SplitCommand(c.line)
		if err != nil {
			t.Errorf("SplitCommand(%q) failed: %v", c.line, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitCommand(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestSplitCommandRefusesWhatNeedsAShell(t *testing.T) {
	for _, line := range []string{
		"a | b", "a && b", "a || b", "a; b", "a > out", "a < in", "a &",
		"echo $(date)", "echo `date`", `echo "$(date)"`, "echo \"`date`\"",
		`echo "unclosed`, "echo 'unclosed", "echo trailing\\", "", "   ",
	} {
		if args, err := SplitCommand(line); err == nil {
			t.Errorf("SplitCommand(%q) = %q, want an error", line, args)
		}
	}
}

func TestUnquotedInlineCode(t *testing.T) {
	flagged := []string{
		"python -c import wordstat",
		"python3 -c from wordstat import cli",
		"python -B -c import wordstat",
		"node -e require ./x.js",
		"sh -c echo hello",
	}
	for _, line := range flagged {
		args, _ := SplitCommand(line)
		if UnquotedInlineCode(args) == "" {
			t.Errorf("%q should be flagged as an unquoted program", line)
		}
	}

	fine := []string{
		`python -c "import wordstat"`,
		`python -c "import sys" extra`,
		"python -c pass",
		"python -m wordstat README.md --top 1",
		"python script.py -c config value",
		"pytest -c pytest.ini tests",
		"go test -c ./pkg",
	}
	for _, line := range fine {
		args, _ := SplitCommand(line)
		if why := UnquotedInlineCode(args); why != "" {
			t.Errorf("%q should not be flagged: %s", line, why)
		}
	}
}

// Quoting must not change what the allowlist sees: the command is matched on
// the arguments that will actually be run.
func TestCheckCommandSeesQuotedArguments(t *testing.T) {
	allow := []string{"go test", "python"}
	if !CheckCommand(`python -c "import wordstat"`, allow) {
		t.Error("a quoted argument made an allowed command fail the allowlist")
	}
	if CheckCommand(`"go test" ./...`, allow) {
		t.Error(`"go test" as one quoted word is a different program and must not match "go test"`)
	}
	if CheckCommand("go test ./... && rm -rf x", allow) {
		t.Error("a command that needs a shell passed the allowlist")
	}
}

func TestRunCmdKeepsQuotedArgumentsTogether(t *testing.T) {
	ts := NewToolSet(t.TempDir(), []string{"sh"})
	ctx := context.Background()

	if out := ts.runCmd(ctx, `sh -c "echo one two"`); strings.TrimSpace(out) != "one two" {
		t.Errorf("quoted program output = %q, want %q", out, "one two")
	}
	if out := ts.runCmd(ctx, "sh -c echo hello"); !strings.Contains(out, "must be ONE quoted argument") {
		t.Errorf("an unquoted program was not explained to the automaton: %s", out)
	}
	if out := ts.runCmd(ctx, "sh -c 'echo a' | cat"); !strings.Contains(out, "needs a shell") {
		t.Errorf("a pipe was not explained to the automaton: %s", out)
	}
}
