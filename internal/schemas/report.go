package schemas

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ReportStatus string

const (
	ReportStatusCompleted   ReportStatus = "COMPLETED"
	ReportStatusBlocked     ReportStatus = "BLOCKED"
	ReportStatusNeedsReview ReportStatus = "NEEDS_REVIEW"
	// ReportStatusAwaitingInput means the worker stopped to ask the user something (see Requests).
	ReportStatusAwaitingInput ReportStatus = "AWAITING_INPUT"
)

func (s ReportStatus) IsValid() bool {
	switch s {
	case ReportStatusCompleted, ReportStatusBlocked, ReportStatusNeedsReview, ReportStatusAwaitingInput:
		return true
	default:
		return false
	}
}

type WorkerReport struct {
	Status       ReportStatus   `yaml:"status" json:"status"`
	Attempts     int            `yaml:"attempts" json:"attempts"`
	Calls        int            `yaml:"calls" json:"calls"`
	TokensIn     int            `yaml:"tokens_in" json:"tokens_in"`
	TokensOut    int            `yaml:"tokens_out" json:"tokens_out"`
	FilesChanged []string       `yaml:"files_changed" json:"files_changed"`
	Requests     []HumanRequest `yaml:"requests,omitempty" json:"requests,omitempty"`

	Summary      string `yaml:"-" json:"summary"`
	Blockers     string `yaml:"-" json:"blockers,omitempty"`
	Verification string `yaml:"-" json:"verification,omitempty"`
}

func ValidateWorkerReport(r *WorkerReport) error {
	if !r.Status.IsValid() {
		return fmt.Errorf("invalid report status '%s': must be COMPLETED, BLOCKED, NEEDS_REVIEW, or AWAITING_INPUT", r.Status)
	}
	if r.Status == ReportStatusAwaitingInput && len(r.Requests) == 0 {
		return fmt.Errorf("report status is AWAITING_INPUT, but 'requests' is empty")
	}
	for _, req := range r.Requests {
		if err := ValidateHumanRequest(req); err != nil {
			return err
		}
	}
	if r.Status == ReportStatusBlocked && strings.TrimSpace(r.Blockers) == "" {
		return fmt.Errorf("report status is BLOCKED, but '## Blockers' section is empty")
	}
	return nil
}

// ParseReportMarkdown reads an automaton's report: YAML frontmatter with at
// least a status, then markdown sections.
//
// The status is the one thing Ringmaster needs from it, and what the report
// claims is checked by the acceptance commands anyway. So a report is read
// as generously as it can be: a code fence around it is ignored, the status
// may be in any letter case, a number written as text or a list written as
// one string does not spoil the rest, and a status line is found even when
// the frontmatter is not valid YAML. Only a report with no recognisable
// status is refused.
func ParseReportMarkdown(content string) (*WorkerReport, error) {
	trimmed := stripCodeFence(strings.TrimSpace(content))
	if trimmed == "" {
		return nil, fmt.Errorf("the report file is empty")
	}

	frontmatter, body := trimmed, ""
	if strings.HasPrefix(trimmed, "---") {
		parts := strings.SplitN(trimmed[3:], "\n---", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("the frontmatter is not closed: there is no second '---' line")
		}
		frontmatter = parts[0]
		body = strings.TrimSpace(strings.TrimLeft(parts[1], "-"))
	} else {
		if !statusLine.MatchString(trimmed) {
			return nil, fmt.Errorf("the report does not start with '---' frontmatter and has no 'status:' line")
		}
		body = trimmed // no frontmatter to cut off: the whole text is the summary
	}

	var report WorkerReport
	report.FilesChanged = []string{}

	if err := yaml.Unmarshal([]byte(frontmatter), &report); err != nil || report.Status == "" {
		report = lenientFrontmatter(frontmatter)
	}
	report.Status = ReportStatus(strings.ToUpper(strings.TrimSpace(string(report.Status))))
	if report.FilesChanged == nil {
		report.FilesChanged = []string{}
	}

	// Extract markdown sections
	report.Summary = body

	if idxBlock := strings.Index(body, "## Blockers"); idxBlock != -1 {
		report.Summary = strings.TrimSpace(body[:idxBlock])
		remaining := body[idxBlock+len("## Blockers"):]
		if idxVer := strings.Index(remaining, "## Verification"); idxVer != -1 {
			report.Blockers = strings.TrimSpace(remaining[:idxVer])
			report.Verification = strings.TrimSpace(remaining[idxVer+len("## Verification"):])
		} else {
			report.Blockers = strings.TrimSpace(remaining)
		}
	} else if idxVer := strings.Index(body, "## Verification"); idxVer != -1 {
		report.Summary = strings.TrimSpace(body[:idxVer])
		report.Verification = strings.TrimSpace(body[idxVer+len("## Verification"):])
	}

	for i := range report.Requests {
		report.Requests[i] = NormalizeRequest(report.Requests[i])
	}

	if err := ValidateWorkerReport(&report); err != nil {
		return nil, err
	}

	return &report, nil
}

func (r *WorkerReport) RenderMarkdown() (string, error) {
	frontBytes, err := yaml.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("failed to marshal report frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(frontBytes)
	sb.WriteString("---\n\n")

	if r.Summary != "" {
		sb.WriteString("## Summary\n")
		sb.WriteString(strings.TrimSpace(r.Summary))
		sb.WriteString("\n\n")
	}

	if r.Blockers != "" {
		sb.WriteString("## Blockers\n")
		sb.WriteString(strings.TrimSpace(r.Blockers))
		sb.WriteString("\n\n")
	}

	if r.Verification != "" {
		sb.WriteString("## Verification\n")
		sb.WriteString(strings.TrimSpace(r.Verification))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

var statusLine = regexp.MustCompile("(?mi)^\\s*status\\s*:\\s*[\"']?([A-Za-z_]+)")

// stripCodeFence removes a ``` fence that wraps the whole report.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if nl := strings.Index(s, "\n"); nl != -1 {
		s = s[nl+1:]
	} else {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

// lenientFrontmatter recovers what it can from frontmatter that does not
// match the report's types: the status above all, then counts and file
// names, each on a best-effort basis.
func lenientFrontmatter(frontmatter string) WorkerReport {
	report := WorkerReport{FilesChanged: []string{}}

	var loose map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &loose); err != nil || loose == nil {
		if m := statusLine.FindStringSubmatch(frontmatter); m != nil {
			report.Status = ReportStatus(m[1])
		}
		return report
	}

	if v, ok := loose["status"]; ok {
		report.Status = ReportStatus(fmt.Sprint(v))
	}
	report.Attempts = looseInt(loose["attempts"])
	report.Calls = looseInt(loose["calls"])
	report.TokensIn = looseInt(loose["tokens_in"])
	report.TokensOut = looseInt(loose["tokens_out"])

	switch files := loose["files_changed"].(type) {
	case []any:
		for _, f := range files {
			report.FilesChanged = append(report.FilesChanged, fmt.Sprint(f))
		}
	case string:
		if strings.TrimSpace(files) != "" {
			report.FilesChanged = append(report.FilesChanged, files)
		}
	}

	// Requests keep their strict shape: a question the user must answer
	// cannot be guessed at
	if raw, ok := loose["requests"]; ok {
		if data, err := yaml.Marshal(raw); err == nil {
			_ = yaml.Unmarshal(data, &report.Requests)
		}
	}
	return report
}

func looseInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
	}
	return 0
}
