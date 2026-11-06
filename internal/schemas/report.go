package schemas

import (
	"fmt"
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

func ParseReportMarkdown(content string) (*WorkerReport, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, fmt.Errorf("missing frontmatter delimiter '---'")
	}

	parts := strings.SplitN(trimmed[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("unterminated frontmatter delimiter '---'")
	}

	frontmatter := parts[0]
	body := strings.TrimSpace(parts[1])

	var report WorkerReport
	report.FilesChanged = []string{}

	if err := yaml.Unmarshal([]byte(frontmatter), &report); err != nil {
		return nil, fmt.Errorf("failed to parse report frontmatter: %w", err)
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
