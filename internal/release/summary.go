package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
	"ringmaster/internal/workspace"
)

// How much of each task's spec and of the diff summary the model is shown:
// enough to tell what was built, without spending the user's quota on it.
const (
	maxSpecChars  = 1500
	maxStatChars  = 3000
	maxSummaryLen = 1200
)

const summarySystemPrompt = `You write the summary at the top of a software release's notes.
Write one to three plain sentences, in English, saying what this release gives its users.
Use only what the facts below say; do not guess at features, numbers or plans.
No heading, no list, no greeting, no quotation marks around the text. Reply with the summary only.`

// SummaryInput is what a release is made of, for the model to summarize.
type SummaryInput struct {
	Project string
	Version string
	Title   string
	Tasks   []NoteTask
	// Commit is where the release ends; the diff summary runs from the last release to it
	Commit string
}

// summaryPrompt gathers the facts: each task's title and spec, and the
// files the release changed.
func summaryPrompt(ctx context.Context, projectRoot string, in SummaryInput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Project: %s\nRelease: %s\n", in.Project, Title(in.Version, in.Title))
	sb.WriteString("\n# Tasks in this release\n")
	for _, t := range in.Tasks {
		fmt.Fprintf(&sb, "\n## %s: %s\n", t.ID, t.Title)
		if spec := taskSpec(projectRoot, t.ID); spec != "" {
			sb.WriteString(spec + "\n")
		}
	}
	if stat := diffStat(ctx, projectRoot, in.Commit); stat != "" {
		sb.WriteString("\n# Files changed\n" + stat + "\n")
	}
	sb.WriteString("\nWrite the summary now.")
	return sb.String()
}

// taskSpec is the body of a task's file, without its frontmatter, shortened.
func taskSpec(projectRoot, id string) string {
	matches, _ := filepath.Glob(filepath.Join(projectRoot, ".ringmaster", "tasks", id+"-*.md"))
	if len(matches) == 0 {
		return ""
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return ""
	}
	// The description only: not the frontmatter, nor the answers given while it ran
	spec, err := schemas.ParseTaskMarkdown(string(data))
	if err != nil {
		return ""
	}
	return shorten(strings.TrimSpace(spec.Description), maxSpecChars)
}

// diffStat lists the files changed since the last release before commit, or
// since the project began when there is none.
func diffStat(ctx context.Context, projectRoot, commit string) string {
	if commit == "" || strings.HasPrefix(commit, "-") {
		return ""
	}
	from := ""
	if out, err := workspace.RunGit(ctx, projectRoot, "describe", "--tags", "--abbrev=0", commit+"^"); err == nil {
		from = strings.TrimSpace(out)
	}
	if from == "" {
		// The empty tree, in the repository's own hash format
		out, err := workspace.RunGit(ctx, projectRoot, "hash-object", "-t", "tree", os.DevNull)
		if err != nil {
			return ""
		}
		from = strings.TrimSpace(out)
	}
	out, err := workspace.RunGit(ctx, projectRoot, "diff", "--stat=100", from, commit)
	if err != nil {
		return ""
	}
	return shorten(strings.TrimSpace(out), maxStatChars)
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Summarize asks the model for the summary of a release. It is called only
// when the user asks for it, since it spends their quota.
func Summarize(ctx context.Context, projectRoot string, provider llm.Provider, model string, in SummaryInput) (string, schemas.Usage, error) {
	var usage schemas.Usage
	if len(in.Tasks) == 0 {
		return "", usage, errors.New("this release holds no tasks to summarize")
	}
	resp, err := provider.Chat(ctx, model, llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: summarySystemPrompt},
		{Role: "user", Content: summaryPrompt(ctx, projectRoot, in)},
	}})
	if err != nil {
		return "", usage, fmt.Errorf("the summary could not be written: %w", err)
	}
	usage.Calls = 1
	usage.TokensIn = resp.Usage.PromptTokens
	usage.TokensCached = resp.Usage.CachedTokens
	usage.TokensOut = resp.Usage.CompletionTokens
	usage.CostUSD = resp.Usage.CostUSD

	summary := cleanSummary(resp.Content)
	if summary == "" {
		return "", usage, errors.New("the model returned an empty summary; write it yourself, or try again")
	}
	return summary, usage, nil
}

// cleanSummary drops what models add around an answer: code fences,
// quotation marks, a "Summary:" label, and anything past a sane length.
func cleanSummary(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```markdown")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	for _, label := range []string{"Summary:", "**Summary:**", "Summary -"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, label))
	}
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"') {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return shorten(s, maxSummaryLen)
}
