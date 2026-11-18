package verify

import (
	"testing"

	"ringmaster/internal/schemas"
)

// Every stop that waits on the user says what kind of answer it needs.
func TestEveryStopSaysWhatAnswerItNeeds(t *testing.T) {
	completed := &schemas.WorkerReport{Status: schemas.ReportStatusCompleted}
	failing := []CheckResult{{Command: "go test ./...", ExitCode: 1}}

	cases := []struct {
		name     string
		decision Decision
		wantType DecisionType
		wantKind schemas.RequestType
	}{
		{"budget", Decide(completed, nil, 1, 2, true), DecisionPAUSED, schemas.RequestChoice},
		{"no report, retries used", Decide(nil, nil, 3, 2, false), DecisionNEED_REVIEW, schemas.RequestChoice},
		{"blocked", Decide(&schemas.WorkerReport{Status: schemas.ReportStatusBlocked, Blockers: "no key"}, nil, 1, 2, false), DecisionBLOCKED, schemas.RequestInput},
		{"automaton asked for review", Decide(&schemas.WorkerReport{Status: schemas.ReportStatusNeedsReview}, nil, 1, 2, false), DecisionNEED_REVIEW, schemas.RequestInput},
		{"unrunnable check", Decide(completed, []CheckResult{{Command: "a | b", Unrunnable: true}}, 1, 2, false), DecisionNEED_REVIEW, schemas.RequestChoice},
		{"check timed out", Decide(completed, []CheckResult{{Command: "docker build .", TimedOut: true}}, 1, 2, false), DecisionNEED_REVIEW, schemas.RequestChoice},
		{"checks still fail", Decide(completed, failing, 3, 2, false), DecisionNEED_REVIEW, schemas.RequestChoice},
		{"unusable report", unusableReportDecision("T01", "no file", 3, 2), DecisionNEED_REVIEW, schemas.RequestChoice},
	}
	for _, c := range cases {
		d := c.decision
		if d.Type != c.wantType {
			t.Errorf("%s: type = %s, want %s", c.name, d.Type, c.wantType)
			continue
		}
		if len(d.Requests) != 1 {
			t.Errorf("%s: %d requests, want 1", c.name, len(d.Requests))
			continue
		}
		r := d.Requests[0]
		if r.ID != NextStepID || r.Type != c.wantKind || r.Question == "" {
			t.Errorf("%s: request = %+v", c.name, r)
		}
		if r.Type == schemas.RequestChoice && (len(r.Options) == 0 || r.Recommended != r.Options[0]) {
			t.Errorf("%s: a choice needs options and a recommended one: %+v", c.name, r)
		}
	}

	// A retry is not a stop: nothing is asked
	if d := Decide(completed, failing, 1, 2, false); d.Type != DecisionRETRY || len(d.Requests) != 0 {
		t.Errorf("retry = %+v", d)
	}
}

func TestNormalizeRequest(t *testing.T) {
	r := schemas.NormalizeRequest(schemas.HumanRequest{ID: "r1", Type: schemas.RequestInput, Question: "Which DB?", Options: []string{"SQLite", "Postgres"}})
	if r.Type != schemas.RequestChoice {
		t.Errorf("a question with options should be a choice: %+v", r)
	}
	r = schemas.NormalizeRequest(schemas.HumanRequest{ID: "r2", Type: schemas.RequestChoice, Question: "Port?", Options: []string{"8080"}})
	if r.Type != schemas.RequestInput || r.Options != nil || r.Recommended != "8080" {
		t.Errorf("a choice with one option should be a question, keeping it as the suggestion: %+v", r)
	}
	r = schemas.NormalizeRequest(schemas.HumanRequest{ID: "r3", Type: schemas.RequestApproval, Command: "make"})
	if r.Type != schemas.RequestApproval {
		t.Errorf("an approval was changed: %+v", r)
	}
}
