#!/bin/sh
# Print release notes, in markdown, for the changes on main's own line
# between two refs, grouped as features, fixes and other changes.
#
# A merged pull request is described by its own commits, whose subjects say
# what changed ("feat: command presets per language"), and linked to the
# pull request; its title is used only when it brought no commits of its
# own. A commit pushed to main directly is described by its subject. Release
# commits are left out.
#
#   scripts/release-notes.sh v0.8.0-hadar HEAD
#
# With an empty <from>, the whole history up to <to> is used.
set -eu

from="${1:-}"
to="${2:-HEAD}"
range="$to"
[ -z "$from" ] || range="$from..$to"

# One line per change, oldest first: "<pull request number>\t<title>"
changes() {
	git log --first-parent --reverse --format='%H %P%x1f%s%x1f%b%x1e' "$range" |
		awk 'BEGIN { RS = "\036"; FS = "\037" } NF >= 2 {
			sub(/^\n+/, "", $1)
			n = split($1, ids, " ")
			# Only the first line of the body: a pull request title, for merges
			m = split($3, lines, "\n"); first = ""
			for (i = 1; i <= m; i++) if (lines[i] ~ /[^ \t]/) { first = lines[i]; break }
			gsub(/\t/, " ", first)
			# hash, first parent, second parent (merges only), subject, title
			# Empty fields are written as "-": read would merge two tabs into one
			printf "%s\t%s\t%s\t%s\t%s\n", ids[1], (n > 1 ? ids[2] : "-"), (n > 2 ? ids[3] : "-"), $2, (first != "" ? first : "-")
		}' |
		while IFS="$(printf '\t')" read -r hash parent merged subject body; do
			case "$subject" in
			"Merge pull request #"*)
				pr=$(printf '%s' "$subject" | sed -n 's/^Merge pull request #\([0-9]*\) .*/\1/p')
				own=""
				[ "$merged" = "-" ] || own=$(git log --no-merges --reverse --format='%s' "$parent..$merged")
				if [ -n "$own" ]; then
					printf '%s\n' "$own" | while IFS= read -r line; do
						[ -n "$line" ] && printf '%s\t%s\n' "$pr" "$line"
					done
				else
					[ "$body" != "-" ] || body="$subject"
					printf '%s\t%s\n' "$pr" "$body"
				fi
				;;
			*) printf '\t%s\n' "$subject" ;;
			esac
		done
}

changes | awk -F '\t' '
	{
		ref = $1; title = $2
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
		if (seen[line]++) next
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
