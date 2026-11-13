package cli

import (
	"strings"
	"testing"

	"ringmaster/internal/remote"
	"ringmaster/internal/workspace"
)

func TestRemoteViewWithoutRemote(t *testing.T) {
	view := buildRemoteView(remote.Status{Name: "origin", Branches: []remote.Branch{
		{Name: "main", State: remote.StateUnpushed},
	}})
	if !strings.Contains(view, "No remote yet") || !strings.Contains(view, "ringmaster remote set") {
		t.Fatalf("view does not say how to set a remote:\n%s", view)
	}
	if strings.Contains(view, "ringmaster push") {
		t.Fatalf("view suggests pushing although there is no remote:\n%s", view)
	}
}

func TestRemoteViewListsWhatNeedsPushing(t *testing.T) {
	view := buildRemoteView(remote.Status{Name: "origin", URL: "git@github.com:you/app.git", Branches: []remote.Branch{
		{Name: "main", State: remote.StateAhead},
		{Name: "rm/T01", State: remote.StateSynced},
		{Name: "rm/T02", State: remote.StateUnpushed},
	}})
	for _, want := range []string{"git@github.com:you/app.git", "has new commits", "not pushed", "2 branch(es) to push"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

func TestRemoteViewWhenEverythingIsPushed(t *testing.T) {
	view := buildRemoteView(remote.Status{Name: "origin", URL: "git@github.com:you/app.git", Branches: []remote.Branch{
		{Name: "main", State: remote.StateSynced},
	}})
	if strings.Contains(view, "to push") {
		t.Fatalf("view asks for a push although everything is pushed:\n%s", view)
	}
}

func TestPushViewMarksRejectedBranches(t *testing.T) {
	view := buildPushView([]remote.PushResult{
		{Branch: "main", Result: remote.ResultRejected},
		{Branch: "rm/T01", Result: remote.ResultNew},
	})
	if !strings.Contains(view, "✗ main") || !strings.Contains(view, "✓ rm/T01") {
		t.Fatalf("unexpected push view:\n%s", view)
	}
}

func TestRemoteViewNamesTheCommitAuthor(t *testing.T) {
	st := remote.Status{Name: "origin", URL: "git@github.com:you/app.git"}
	if view := buildRemoteView(st); !strings.Contains(view, "author  Ringmaster") || !strings.Contains(view, "user.name") {
		t.Fatalf("view does not warn that commits are authored by Ringmaster:\n%s", view)
	}

	st.Author = workspace.Author{Name: "Ada Lovelace", Email: "ada@example.com", IsUser: true}
	if view := buildRemoteView(st); !strings.Contains(view, "author  Ada Lovelace <ada@example.com>") {
		t.Fatalf("view does not name the author:\n%s", view)
	}
}
