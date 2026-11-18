package worker

import (
	"fmt"

	"ringmaster/internal/config"
)

// Permission is whether an automaton may run a command.
type Permission int

const (
	// Allowed: run it.
	Allowed Permission = iota
	// Ask: an allowed entry covers it, but it is on the always-ask list
	// (npm publish under npm); the user must approve it first.
	Ask
	// NotAllowed: no allowed entry covers it; the user must approve it first.
	NotAllowed
)

// CommandPermission decides whether cmd may run in sb. A command the user
// approved runs; otherwise one on the always-ask list stops to ask, even when
// an allowed entry covers it; otherwise it runs only if allowed. The reason
// says which rule applied, for the automaton and the user.
func CommandPermission(cmd string, sb config.SandboxConfig) (Permission, string) {
	if _, err := SplitCommand(cmd); err != nil {
		return NotAllowed, err.Error()
	}
	if CheckCommand(cmd, sb.Approved) {
		return Allowed, "approved by the user"
	}
	if entry := matchingEntry(cmd, sb.AlwaysAsk); entry != "" {
		return Ask, fmt.Sprintf("'%s' always needs the user's approval", entry)
	}
	if CheckCommand(cmd, sb.Allow) {
		return Allowed, "on the allowlist"
	}
	return NotAllowed, "not on the allowlist"
}

// matchingEntry returns the first entry of list that covers cmd, or "".
func matchingEntry(cmd string, list []string) string {
	for _, entry := range list {
		if CheckCommand(cmd, []string{entry}) {
			return entry
		}
	}
	return ""
}

// RunnableCommands is what an agentic provider may run on its own: the
// allowlist and the user's approvals. The always-ask entries not covered by
// an approval are returned separately, to be denied.
func RunnableCommands(sb config.SandboxConfig) (allowed, denied []string) {
	allowed = append(append([]string{}, sb.Allow...), sb.Approved...)
	for _, entry := range sb.AlwaysAsk {
		if !CheckCommand(entry, sb.Approved) {
			denied = append(denied, entry)
		}
	}
	return allowed, denied
}
