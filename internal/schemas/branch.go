package schemas

import (
	"regexp"
	"strings"
)

// LegacyBranchPrefix is the namespace task branches used before they had
// readable names ("rm/T01"). Projects that already have such branches keep them.
const LegacyBranchPrefix = "rm/"

// LegacyBranch is the branch a task had before branches were named.
func LegacyBranch(taskID string) string {
	return LegacyBranchPrefix + taskID
}

// BranchPrefixes are the kinds of change a task branch may announce, the way
// people name branches by hand: feat/link-store, fix/empty-file, docs/usage.
var BranchPrefixes = []string{"feat", "fix", "docs", "test", "refactor", "chore"}

const maxBranchSlug = 40

var (
	notSlug    = regexp.MustCompile(`[^a-z0-9]+`)
	stopWords  = map[string]bool{"a": true, "an": true, "the": true, "and": true, "of": true, "for": true, "to": true, "in": true, "with": true}
	prefixLike = map[string]string{"feature": "feat", "features": "feat", "bugfix": "fix", "hotfix": "fix", "doc": "docs", "tests": "test", "ci": "chore", "build": "chore", "infra": "chore"}
)

// BranchName returns the branch for a task: "<prefix>/<slug>", lower-case,
// slug of letters, digits and dashes. proposed is what the planner suggested;
// whatever in it is usable is kept, and the rest comes from the task's type
// and title. The result is always a valid branch name.
func BranchName(proposed string, taskType TaskType, title string) string {
	prefix, slug := "", ""

	p := strings.ToLower(strings.TrimSpace(proposed))
	// People write feat:thing as well as feat/thing; a colon cannot be in a ref
	p = strings.Replace(p, ":", "/", 1)
	if head, rest, ok := strings.Cut(p, "/"); ok {
		head = strings.Trim(notSlug.ReplaceAllString(head, "-"), "-")
		if mapped, known := prefixLike[head]; known {
			head = mapped
		}
		if isBranchPrefix(head) {
			prefix = head
		}
		slug = slugify(rest)
	} else {
		slug = slugify(p)
	}

	if prefix == "" {
		prefix = prefixForType(taskType)
	}
	if slug == "" {
		slug = slugify(title)
	}
	if slug == "" {
		slug = "task"
	}
	return prefix + "/" + slug
}

func isBranchPrefix(s string) bool {
	for _, p := range BranchPrefixes {
		if s == p {
			return true
		}
	}
	return false
}

func prefixForType(t TaskType) string {
	switch t {
	case TaskTypeDocs:
		return "docs"
	case TaskTypeTest:
		return "test"
	case TaskTypeInfra:
		return "chore"
	default:
		return "feat"
	}
}

// slugify makes a short dash-separated slug, dropping filler words and
// cutting at a word boundary.
func slugify(s string) string {
	words := strings.Split(strings.Trim(notSlug.ReplaceAllString(strings.ToLower(s), "-"), "-"), "-")
	var kept []string
	length := 0
	for _, w := range words {
		if w == "" || (stopWords[w] && len(words) > 2) {
			continue
		}
		if length+len(w)+1 > maxBranchSlug && len(kept) > 0 {
			break
		}
		if len(w) > maxBranchSlug {
			w = w[:maxBranchSlug]
		}
		kept = append(kept, w)
		length += len(w) + 1
	}
	return strings.Join(kept, "-")
}
