package worker

import (
	"strings"
	"testing"

	"ringmaster/internal/config"
)

func TestCommandPermission(t *testing.T) {
	sb := config.SandboxConfig{
		Allow:     []string{"npm", "docker compose", "go test", "git"},
		AlwaysAsk: []string{"npm publish", "docker compose down", "git push"},
		Approved:  []string{"docker compose down --remove-orphans"},
	}
	cases := []struct {
		cmd  string
		want Permission
	}{
		{"npm test", Allowed},
		{"npm run build", Allowed},
		{"npm publish", Ask},
		{"npm publish --access public", Ask},
		{"docker compose up -d", Allowed},
		{"docker compose down -v", Ask},
		// The user approved this exact form
		{"docker compose down --remove-orphans", Allowed},
		{"git status", Allowed},
		{"git push origin main", Ask},
		{"go test ./...", Allowed},
		{"go build ./...", NotAllowed},
		{"rm -rf /", NotAllowed},
		{"npm test && npm publish", NotAllowed},
		{`npm "publish"`, Ask}, // quoting does not hide the command
	}
	for _, c := range cases {
		if got, why := CommandPermission(c.cmd, sb); got != c.want {
			t.Errorf("CommandPermission(%q) = %v (%s), want %v", c.cmd, got, why, c.want)
		}
	}
	if _, why := CommandPermission("npm publish", sb); !strings.Contains(why, "'npm publish' always needs the user's approval") {
		t.Errorf("reason = %q", why)
	}
}

func TestRunnableCommands(t *testing.T) {
	sb := config.SandboxConfig{
		Allow:     []string{"npm"},
		AlwaysAsk: []string{"npm publish", "git push"},
		Approved:  []string{"git push"},
	}
	allowed, denied := RunnableCommands(sb)
	if strings.Join(allowed, ",") != "npm,git push" {
		t.Errorf("allowed = %v", allowed)
	}
	if strings.Join(denied, ",") != "npm publish" {
		t.Errorf("denied = %v (an approved always-ask command must not be denied)", denied)
	}
}

func TestRunCmdStopsForAlwaysAskCommands(t *testing.T) {
	ts := NewToolSet(t.TempDir(), []string{"sh"})
	ts.AlwaysAsk = []string{"sh -c"}
	out := ts.runCmd(t.Context(), `sh -c "echo should not run"`)
	// The command is quoted in the error; it must not have run
	if !strings.HasPrefix(out, "Error:") || !strings.Contains(out, "always needs the user's approval") || !strings.Contains(out, "request approval") {
		t.Errorf("output = %q", out)
	}
	ts.Approved = []string{"sh -c"}
	if out := ts.runCmd(t.Context(), `sh -c "echo ran"`); strings.TrimSpace(out) != "ran" {
		t.Errorf("an approved command did not run: %q", out)
	}
}
