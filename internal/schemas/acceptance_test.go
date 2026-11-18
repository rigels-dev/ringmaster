package schemas

import (
	"slices"
	"strings"
	"testing"
)

const taskWithAnswers = `---
id: T13
title: "Dockerfile and compose.yaml"
type: infra
depends_on: ["T12"]
worker: "haiku"
max_retries: 2
acceptance:
  - "docker build -t shorty:test ."
  - "docker compose config --quiet"
  - "go test ./..."
acceptance_timeout: 900
---

Add container support at the repository root.

## Human Answers

### Answer (2026-11-18T08:17:32Z)
- [next] What should happen? → just make dockerfile and dockercompose only
`

func TestReplaceAcceptanceKeepsTheRestOfTheFile(t *testing.T) {
	got, err := ReplaceAcceptance(taskWithAnswers, []string{"go test ./...", `grep -q "FROM golang" Dockerfile`})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := ParseTaskMarkdown(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go test ./...", `grep -q "FROM golang" Dockerfile`}; !slices.Equal(spec.Acceptance, want) {
		t.Errorf("acceptance = %q, want %q", spec.Acceptance, want)
	}
	if spec.AcceptanceTimeout != 900 || spec.MaxRetries != 2 || spec.Worker != "haiku" || !slices.Equal(spec.DependsOn, []string{"T12"}) {
		t.Errorf("other frontmatter changed: %+v", spec)
	}
	for _, keep := range []string{"Add container support", "## Human Answers", "just make dockerfile"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q:\n%s", keep, got)
		}
	}
	if strings.Contains(got, "docker build") {
		t.Errorf("the old command is still there:\n%s", got)
	}
}

func TestReplaceAcceptanceInlineAndMissingList(t *testing.T) {
	inline := "---\nid: T01\ntitle: \"x\"\ntype: backend\nacceptance: [\"make\"]\nworker: \"w\"\n---\nBody\n"
	got, err := ReplaceAcceptance(inline, []string{"go vet ./..."})
	if err != nil {
		t.Fatal(err)
	}
	if spec, _ := ParseTaskMarkdown(got); spec == nil || !slices.Equal(spec.Acceptance, []string{"go vet ./..."}) || spec.Worker != "w" {
		t.Errorf("inline list not replaced:\n%s", got)
	}

	none := "---\nid: T01\ntitle: \"x\"\ntype: backend\nworker: \"w\"\n---\nBody\n"
	if got, err = ReplaceAcceptance(none, []string{"go vet ./..."}); err != nil || !strings.Contains(got, "acceptance:\n  - \"go vet ./...\"") {
		t.Errorf("a missing list was not added: %v\n%s", err, got)
	}

	if _, err := ReplaceAcceptance(taskWithAnswers, nil); err == nil {
		t.Error("an empty list was accepted")
	}
}
