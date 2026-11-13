package verify

import "testing"

func TestUnrunnableReason(t *testing.T) {
	runnable := []string{
		"pytest -q",
		"go test ./...",
		"npm test",
		"python -m wordstat README.md --top 1",
		"node --test tests/",
	}
	for _, cmd := range runnable {
		if why := UnrunnableReason(cmd); why != "" {
			t.Errorf("UnrunnableReason(%q) = %q, want none", cmd, why)
		}
	}

	needShell := []string{
		`python -c "import pathlib; assert pathlib.Path('README.md').exists()"`,
		"grep -q json README.md && echo ok",
		"cat README.md | grep json",
		"test -f README.md; echo done",
		"echo $(date)",
		"go test ./... > out.txt",
		"grep 'python -m wordstat' README.md",
	}
	for _, cmd := range needShell {
		if why := UnrunnableReason(cmd); why == "" {
			t.Errorf("UnrunnableReason(%q) = none, want a reason", cmd)
		}
	}
}
