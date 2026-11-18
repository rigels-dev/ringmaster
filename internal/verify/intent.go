package verify

import "ringmaster/internal/schemas"

// Every stop that waits on the user says what kind of answer it needs: a pick
// among a few options, which the user may still replace with their own
// words, or a written answer. The answer is recorded under the task's human
// answers, so the next attempt reads it.

// NextStepID is the id of the request Ringmaster adds when a task stops.
const NextStepID = "next"

// choose asks the user to pick what happens next.
func choose(question string, options ...string) []schemas.HumanRequest {
	return []schemas.HumanRequest{{
		ID:          NextStepID,
		Type:        schemas.RequestChoice,
		Question:    question,
		Options:     options,
		Recommended: options[0],
	}}
}

// ask needs a written answer: something only the user can say.
func ask(question string) []schemas.HumanRequest {
	return []schemas.HumanRequest{{
		ID:       NextStepID,
		Type:     schemas.RequestInput,
		Question: question,
	}}
}

// Options offered when a task stops. They are written to be read by the
// next attempt as much as by the user.
const (
	OptionContinueBudget  = "Continue with another full token budget"
	OptionFinishBudget    = "Continue, but only finish what is started and write the report"
	OptionRetryChecks     = "Try again: read the failing check output and fix the code"
	OptionRetryDifferent  = "Try again with a different approach"
	OptionReportOnly      = "The work is done: do not change any file, write only the report"
	OptionRetryTask       = "Try the task again"
	OptionFixedAcceptance = "I replaced the acceptance command in the task's file: try again"
	OptionRaisedTimeout   = "I raised acceptance_timeout: try again"
	OptionRetryAsIs       = "Try again as it is"
)

// What a stopped task asks the user, for stops that are not already
// questions of the automaton's own.
func budgetRequests() []schemas.HumanRequest {
	return choose("The task used its token budget before it finished. How should it continue?",
		OptionContinueBudget, OptionFinishBudget)
}

func checksFailedRequests() []schemas.HumanRequest {
	return choose("The acceptance checks still fail after every attempt. What should the automaton do?",
		OptionRetryChecks, OptionRetryDifferent)
}

func unusableReportRequests() []schemas.HumanRequest {
	return choose("The automaton finished without a report Ringmaster could use. What next?",
		OptionReportOnly, OptionRetryTask)
}

// UnrunnableAcceptanceRequests asks the user to fix an acceptance command
// that can never run: no attempt can.
func UnrunnableAcceptanceRequests() []schemas.HumanRequest {
	return choose("This acceptance command cannot run, so no attempt can pass it. Replace it in the task's file "+
		"(.ringmaster/tasks/), or re-plan in Chat instead of answering.", OptionFixedAcceptance)
}

func timeoutRequests() []schemas.HumanRequest {
	return choose("An acceptance check ran out of time.", OptionRaisedTimeout, OptionRetryAsIs)
}

func blockedRequests() []schemas.HumanRequest {
	return ask("The automaton is blocked. What does it need from you to continue?")
}

func reviewRequests() []schemas.HumanRequest {
	return ask("The automaton stopped and asked for a review. What should it do next?")
}

// MergeConflictRequests asks how to go on when a branch would not merge.
func MergeConflictRequests() []schemas.HumanRequest {
	return ask("The task's branch could not be merged into main. Resolve the conflict on the branch, " +
		"or tell the automaton what to keep, then answer.")
}
