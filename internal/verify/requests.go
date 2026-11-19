package verify

import (
	"fmt"
	"slices"
	"strings"

	"ringmaster/internal/schemas"
	"ringmaster/internal/worker"
)

// TestEditsID is the id of the request Ringmaster adds when a task that may
// not change existing tests stops over one of them.
const TestEditsID = "tests"

// TestEditsRequest asks whether the task may change the existing tests it
// touched or asked about. Only the user can allow that, and only Ringmaster
// can apply it: an answer to the automaton cannot, since Ringmaster puts
// protected tests back before every check.
func TestEditsRequest(files []string) schemas.HumanRequest {
	return schemas.HumanRequest{
		ID:   TestEditsID,
		Type: schemas.RequestChoice,
		Question: fmt.Sprintf("This task may not change existing tests, and it stopped over %s. "+
			"If a test is outdated for this task, let the task change it: Ringmaster then keeps the automaton's test changes. "+
			"Telling the automaton to edit it is not enough, because Ringmaster restores protected tests before every check.",
			strings.Join(files, ", ")),
		Options:     []string{OptionAllowTestEdits, OptionKeepTests},
		Recommended: OptionKeepTests,
	}
}

// AllowsTestEdits reports whether an answer lets the task change existing tests.
func AllowsTestEdits(ans schemas.HumanAnswer) bool {
	for _, reply := range ans.Replies {
		if reply == OptionAllowTestEdits {
			return true
		}
	}
	return false
}

// CheckRequests fixes what an automaton asked for that approving could not
// give: an "approval" of something that is not a command (an instruction
// in words, say), or of a command the user has approved already. Approving
// either changes nothing, so the automaton would ask again and again. Such a
// request becomes a question, answered in words.
func CheckRequests(requests []schemas.HumanRequest, approved func(command string) bool) []schemas.HumanRequest {
	out := make([]schemas.HumanRequest, 0, len(requests))
	for _, r := range requests {
		if r.Type == schemas.RequestApproval {
			command := strings.Join(strings.Fields(r.Command), " ")
			var why string
			switch {
			case notACommand(command) != "":
				why = fmt.Sprintf("The automaton asked to approve %q, but %s, so approving it would change nothing.", command, notACommand(command))
			case approved != nil && approved(command):
				why = fmt.Sprintf("The automaton asked again to run `%s`, which you have approved already. It may be unable to run it at all.", command)
			}
			if why != "" {
				question := why + " What should it do instead?"
				if reason := strings.TrimSpace(r.Reason); reason != "" {
					question += " (Its reason: " + reason + ")"
				}
				r = schemas.HumanRequest{ID: r.ID, Type: schemas.RequestInput, Question: question}
			}
		}
		out = append(out, r)
	}
	return out
}

// notACommand says why command is not something to run, or "".
func notACommand(command string) string {
	args, err := worker.SplitCommand(command)
	if err != nil || len(args) == 0 {
		return "it cannot be read as a shell command"
	}
	program := args[0]
	// A path to a script in the workspace is a command, even before it exists
	if strings.Contains(program, "/") {
		return ""
	}
	if MissingProgram(command) != "" {
		return fmt.Sprintf("'%s' is not a program on this machine", program)
	}
	return ""
}

// MentionedTestFiles lists the test files the requests name, in their
// question, reason or command.
func MentionedTestFiles(requests []schemas.HumanRequest, patterns []string) []string {
	files := []string{}
	for _, r := range requests {
		text := r.Question + " " + r.Reason + " " + r.Command
		for _, word := range strings.FieldsFunc(text, func(c rune) bool {
			return strings.ContainsRune(" \t\n,;:()[]{}<>`'\"", c)
		}) {
			word = strings.TrimSuffix(word, ".")
			// A file name has an extension; "tests" alone is a word
			if !strings.Contains(word, ".") || !worker.IsTestFile(word, patterns) {
				continue
			}
			if !slices.Contains(files, word) {
				files = append(files, word)
			}
		}
	}
	return files
}
