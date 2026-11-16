package schemas

import (
	"regexp"
	"testing"
)

func TestBranchName(t *testing.T) {
	cases := []struct {
		proposed string
		taskType TaskType
		title    string
		want     string
	}{
		// What the planner is asked for is kept as it is
		{"feat/link-store", TaskTypeBackend, "Thread-safe link store", "feat/link-store"},
		{"fix/empty-file-exit-code", TaskTypeBackend, "x", "fix/empty-file-exit-code"},
		// People also write a colon; a ref cannot hold one
		{"feat:whatever", TaskTypeBackend, "x", "feat/whatever"},
		{"Feature/Add Login Page!", TaskTypeFrontend, "x", "feat/add-login-page"},
		{"bugfix/crash_on_start", TaskTypeBackend, "x", "fix/crash-on-start"},
		// An unknown prefix gives way to the task's type; the slug is kept
		{"wip/link-store", TaskTypeBackend, "x", "feat/link-store"},
		{"link-store", TaskTypeDocs, "x", "docs/link-store"},
		// Nothing usable: built from type and title
		{"", TaskTypeBackend, "Go module and thread-safe in-memory link store", "feat/go-module-thread-safe-memory-link-store"},
		{"", TaskTypeDocs, "README usage documentation", "docs/readme-usage-documentation"},
		{"", TaskTypeTest, "Concurrency tests", "test/concurrency-tests"},
		{"", TaskTypeInfra, "CI pipeline", "chore/ci-pipeline"},
		{"///", TaskTypeBackend, "", "feat/task"},
		// Nothing that git could misread survives
		{"--force", TaskTypeBackend, "x", "feat/force"},
		{"feat/../../etc/passwd", TaskTypeBackend, "x", "feat/etc-passwd"},
		{"feat/x b~c^d:e?f*g[h", TaskTypeBackend, "x", "feat/x-b-c-d-e-f-g-h"},
	}
	valid := regexp.MustCompile(`^(feat|fix|docs|test|refactor|chore)/[a-z0-9]+(-[a-z0-9]+)*$`)
	for _, c := range cases {
		got := BranchName(c.proposed, c.taskType, c.title)
		if got != c.want {
			t.Errorf("BranchName(%q, %s, %q) = %q, want %q", c.proposed, c.taskType, c.title, got, c.want)
		}
		if !valid.MatchString(got) {
			t.Errorf("BranchName(%q, ...) = %q is not a clean branch name", c.proposed, got)
		}
	}
}

func TestBranchSlugStaysShort(t *testing.T) {
	got := BranchName("", TaskTypeBackend, "Implement the complete and extremely thorough validation of every incoming request payload field")
	if len(got) > len("feat/")+maxBranchSlug {
		t.Errorf("branch %q is longer than the limit", got)
	}
	if got[len(got)-1] == '-' {
		t.Errorf("branch %q ends with a dash", got)
	}
}
