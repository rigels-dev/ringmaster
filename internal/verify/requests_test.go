package verify

import (
	"strings"
	"testing"

	"ringmaster/internal/schemas"
	"ringmaster/internal/worker"
)

func TestCheckRequestsTurnsUselessApprovalsIntoQuestions(t *testing.T) {
	sentence := "remove the {GET, /api/links} case from TestE2EWrongMethodIsNot2xx in internal/httpapi/e2e_test.go (outside this run), then re-run T07"
	approved := func(cmd string) bool { return cmd == "go version" }
	got := CheckRequests([]schemas.HumanRequest{
		{ID: "r1", Type: schemas.RequestApproval, Command: sentence, Reason: "the test is outdated"},
		{ID: "r2", Type: schemas.RequestApproval, Command: "go  version"},
		{ID: "r3", Type: schemas.RequestApproval, Command: "go env GOPATH"},
		{ID: "r4", Type: schemas.RequestApproval, Command: "./scripts/gen.sh --all"},
		{ID: "r5", Type: schemas.RequestInput, Question: "Which port?"},
	}, approved)

	if got[0].Type != schemas.RequestInput || got[0].Command != "" ||
		!strings.Contains(got[0].Question, "'remove' is not a program") || !strings.Contains(got[0].Question, "the test is outdated") {
		t.Errorf("an instruction in words is still an approval: %+v", got[0])
	}
	if got[1].Type != schemas.RequestInput || !strings.Contains(got[1].Question, "approved already") {
		t.Errorf("a command approved already is asked again: %+v", got[1])
	}
	// A real command, and a script in the workspace, stay approvals
	for _, r := range got[2:4] {
		if r.Type != schemas.RequestApproval {
			t.Errorf("request %s changed: %+v", r.ID, r)
		}
	}
	if got[4].Question != "Which port?" {
		t.Errorf("a question changed: %+v", got[4])
	}
}

func TestMentionedTestFiles(t *testing.T) {
	got := MentionedTestFiles([]schemas.HumanRequest{
		{Question: "internal/httpapi/e2e_test.go, TestE2EWrongMethodIsNot2xx, expects GET /api/links to return non-2xx."},
		{Command: "remove a case from `internal/httpapi/e2e_test.go`", Reason: "tests are outdated; see web/app.test.ts."},
		{Question: "Should main.go change?"},
	}, worker.DefaultTestFiles)
	if strings.Join(got, ",") != "internal/httpapi/e2e_test.go,web/app.test.ts" {
		t.Fatalf("files: %v", got)
	}
}

func TestAllowsTestEdits(t *testing.T) {
	if AllowsTestEdits(schemas.HumanAnswer{Replies: map[string]string{TestEditsID: OptionKeepTests}}) {
		t.Error("keeping the tests allows edits")
	}
	if !AllowsTestEdits(schemas.HumanAnswer{Replies: map[string]string{TestEditsID: OptionAllowTestEdits}}) {
		t.Error("the allow option does not allow edits")
	}
	r := TestEditsRequest([]string{"a_test.go"})
	if r.Type != schemas.RequestChoice || r.Options[0] != OptionAllowTestEdits || !strings.Contains(r.Question, "a_test.go") {
		t.Errorf("request: %+v", r)
	}
	if err := schemas.ValidateHumanRequest(r); err != nil {
		t.Error(err)
	}
}
