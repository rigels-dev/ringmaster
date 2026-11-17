#!/bin/sh
# Print release notes, in markdown, for the changes on main's own line
# between two refs: the pull requests merged and the commits pushed
# directly, grouped as features, fixes and other changes. Release commits
# themselves are left out.
#
#   scripts/release-notes.sh v0.8.0-hadar HEAD
#
# With an empty <from>, the whole history up to <to> is used.
set -eu

from="${1:-}"
to="${2:-HEAD}"
range="$to"
[ -z "$from" ] || range="$from..$to"

# One record per commit: subject and body, separated by \037, records by \036
git log --first-parent --format='%s%x1f%b%x1e' "$range" | awk '
	BEGIN { RS = "\036"; FS = "\037" }
	{
		subject = $1; body = $2
		sub(/^\n+/, "", subject)
		title = subject; ref = ""
		# A merged pull request is named by its title, the first line of the merge body
		if (match(subject, /^Merge pull request #[0-9]+ /)) {
			ref = substr(subject, 21, RLENGTH - 21)
			n = split(body, lines, "\n")
			title = ""
			for (i = 1; i <= n; i++) if (lines[i] != "") { title = lines[i]; break }
			if (title == "") title = subject
		}
		if (title == "" || title ~ /^chore: release /) next

		# Conventional commit prefix: type, optional (scope), optional !
		group = "other"; text = title; type = ""
		if (match(title, /^[a-z]+(\([^)]*\))?!?: /)) {
			type = substr(title, 1, RLENGTH)
			sub(/[(!:].*/, "", type)
			text = substr(title, RLENGTH + 1)
		} else if (match(title, /^[A-Za-z]+\//)) {
			# A pull request titled after its branch, as GitHub does by default: "Feat/rounds and branch names"
			type = tolower(substr(title, 1, RLENGTH - 1))
			text = substr(title, RLENGTH + 1)
		}
		# Release pull requests, however they are titled, are not changes
		if (type == "chore" && tolower(text) ~ /^release /) next
		if (type == "feat" || type == "feature") group = "feat"
		else if (type == "fix" || type == "bugfix" || type == "hotfix") group = "fix"
		text = toupper(substr(text, 1, 1)) substr(text, 2)
		line = "- " text (ref != "" ? " (#" ref ")" : "")
		items[group] = items[group] line "\n"
	}
	END {
		out = ""
		if (items["feat"] != "") out = out "### Features\n" items["feat"]
		if (items["fix"] != "") out = out (out != "" ? "\n" : "") "### Fixes\n" items["fix"]
		if (items["other"] != "") out = out (out != "" ? "\n" : "") "### Other changes\n" items["other"]
		printf "%s", out
	}
'
