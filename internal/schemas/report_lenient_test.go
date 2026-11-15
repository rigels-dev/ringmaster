package schemas

import (
	"strings"
	"testing"
)

func TestReportIsReadGenerously(t *testing.T) {
	cases := map[string]string{
		"as specified":            "---\nstatus: COMPLETED\nattempts: 1\ncalls: 0\ntokens_in: 0\ntokens_out: 0\nfiles_changed: []\n---\n\n## Summary\nDone.",
		"numbers written as text": "---\nstatus: COMPLETED\nattempts: \"1 of 3\"\ncalls: n/a\ntokens_in: unknown\ntokens_out: \"12\"\nfiles_changed: []\n---\n\n## Summary\nDone.",
		"files as one string":     "---\nstatus: COMPLETED\nattempts: 1\nfiles_changed: README.md\n---\n\n## Summary\nDone.",
		"lower-case status":       "---\nstatus: completed\n---\n\n## Summary\nDone.",
		"quoted status":           "---\nstatus: \"COMPLETED\"\n---\n\n## Summary\nDone.",
		"wrapped in a code fence": "```markdown\n---\nstatus: COMPLETED\nattempts: 1\n---\n\n## Summary\nDone.\n```",
		"only a status":           "---\nstatus: COMPLETED\n---\nDone.",
		"extra unknown fields":    "---\nstatus: COMPLETED\nduration: 5m\nnotes: [a, b]\n---\n\n## Summary\nDone.",
		"frontmatter not YAML":    "---\nstatus: COMPLETED\nsummary: wrote the README: usage, exit codes\n\tbad indent: [\n---\n\n## Summary\nDone.",
		"no frontmatter at all":   "# Report for T04\n\nstatus: COMPLETED\n\nDone.",
		"dashes inside a value":   "---\nstatus: COMPLETED\nfiles_changed: [\"a---b.md\"]\n---\n\n## Summary\nDone.",
	}
	for name, content := range cases {
		report, err := ParseReportMarkdown(content)
		if err != nil {
			t.Errorf("%s: refused: %v", name, err)
			continue
		}
		if report.Status != ReportStatusCompleted {
			t.Errorf("%s: status = %q, want COMPLETED", name, report.Status)
		}
		if !strings.Contains(report.Summary, "Done.") {
			t.Errorf("%s: summary lost: %q", name, report.Summary)
		}
		if report.FilesChanged == nil {
			t.Errorf("%s: files_changed is nil", name)
		}
	}

	report, _ := ParseReportMarkdown(cases["numbers written as text"])
	if report.TokensOut != 12 || report.Attempts != 0 {
		t.Errorf("counts: tokens_out=%d attempts=%d, want 12 and 0", report.TokensOut, report.Attempts)
	}
	report, _ = ParseReportMarkdown(cases["files as one string"])
	if len(report.FilesChanged) != 1 || report.FilesChanged[0] != "README.md" {
		t.Errorf("files_changed = %v", report.FilesChanged)
	}
}

// What cannot be read as a statement that the work is finished stays refused,
// with a reason that says what is wrong.
func TestReportWithoutAUsableStatusIsRefused(t *testing.T) {
	cases := map[string]string{
		"empty file":             "",
		"prose only":             "I finished the task and everything passes.",
		"unknown status":         "---\nstatus: FINISHED_OK\n---\nDone.",
		"frontmatter not closed": "---\nstatus: COMPLETED\n\n## Summary\nDone.",
		"asks without requests":  "---\nstatus: AWAITING_INPUT\n---\nI need the API key.",
	}
	for name, content := range cases {
		if report, err := ParseReportMarkdown(content); err == nil {
			t.Errorf("%s: accepted as %q", name, report.Status)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: refused without a reason", name)
		}
	}
}

// A question for the user must still arrive whole.
func TestRequestsSurviveLooseFrontmatter(t *testing.T) {
	content := "---\nstatus: awaiting_input\nattempts: one\nrequests:\n  - id: r1\n    type: input\n    question: Which region?\n---\n\n## Summary\nNeed a region."
	report, err := ParseReportMarkdown(content)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != ReportStatusAwaitingInput || len(report.Requests) != 1 || report.Requests[0].Question != "Which region?" {
		t.Fatalf("report = %+v", report)
	}
}
