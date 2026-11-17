#!/bin/sh
# Prepare a release: a branch chore/release-<version> with VERSION and the
# README's "Current release" line updated, committed and pushed, ready for a
# pull request. Once that is merged and pulled, 'make release' tags it.
#
#   scripts/bump.sh patch               v0.8.0-hadar -> v0.8.1-hadar  (fixes)
#   scripts/bump.sh minor               v0.8.1-hadar -> v0.9.0-izar   (features)
#   scripts/bump.sh major               v0.9.0-izar  -> v1.0.0-jabbah (breaking)
#   scripts/bump.sh v0.9.0-izar         an exact version
#
# A minor or major release moves the codename to the next star in
# internal/version/version.go; a patch release keeps it.
#
# The release notes are written from the pull requests merged since the last
# release (scripts/release-notes.sh) and go into the release commit, the pull
# request and, through 'make release', the GitHub release. NOTES adds a
# summary at the top:
#
#   NOTES="Steps can be released from Branches." scripts/bump.sh minor
set -eu

kind="${1:-}"
remote="${RELEASE_REMOTE:-origin}"

usage() {
	echo "usage: scripts/bump.sh patch|minor|major|<version>" >&2
	exit 2
}

step() {
	echo "[$1/5] $2"
}

fail() {
	echo "bump: $1" >&2
	exit 1
}

# urlencode percent-encodes everything but unreserved characters, byte by
# byte, with tools every system has
urlencode() {
	printf '%s' "$1" | od -An -v -tx1 | tr -s ' \n' '\n\n' | awk '
		NF {
			h = tolower($1)
			d = (index("0123456789abcdef", substr(h, 1, 1)) - 1) * 16 + index("0123456789abcdef", substr(h, 2, 1)) - 1
			if ((d >= 48 && d <= 57) || (d >= 65 && d <= 90) || (d >= 97 && d <= 122) || d == 45 || d == 46 || d == 95 || d == 126)
				printf "%c", d
			else
				printf "%%%s", toupper(h)
		}'
}

[ -n "$kind" ] || usage

step 1 "Checking the local checkout"
branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || fail "you are on '$branch'. Run: git checkout main && git pull"
git diff --quiet HEAD || fail "there are uncommitted changes. Commit or stash them first"
current=$(tr -d ' \n' < VERSION)

# The codenames, in order, from the one list the version test also uses
codenames=$(sed -n '/^var Codenames/,/^}/p' internal/version/version.go | grep -o '"[a-z]*"' | tr -d '"')

next_codename() {
	printf '%s\n' "$codenames" | awk -v cur="$1" 'found { print; exit } $0 == cur { found = 1 }'
}

case "$kind" in
patch | minor | major)
	parts=$(printf '%s\n' "$current" | sed -n 's/^v\([0-9]*\)\.\([0-9]*\)\.\([0-9]*\)-\([a-z]*\)$/\1 \2 \3 \4/p')
	[ -n "$parts" ] || fail "VERSION holds '$current', which is not v{major}.{mid}.{minor}-{codename}"
	set -- $parts
	major=$1 mid=$2 minor=$3 codename=$4
	case "$kind" in
	patch) minor=$((minor + 1)) ;;
	minor) mid=$((mid + 1)) minor=0 ;;
	major) major=$((major + 1)) mid=0 minor=0 ;;
	esac
	if [ "$kind" != patch ]; then
		codename=$(next_codename "$codename")
		[ -n "$codename" ] || fail "there is no codename after '$4' in internal/version/version.go"
	fi
	version="v$major.$mid.$minor-$codename"
	;;
v*) version="$kind" ;;
*) usage ;;
esac

[ "$version" != "$current" ] || fail "VERSION is already $version"
release_branch="chore/release-$version"
echo "      $current -> $version"

step 2 "Asking $remote for main, and whether $version exists (connects to the remote)"
refs=$(git ls-remote "$remote" refs/heads/main "refs/heads/$release_branch" "refs/tags/$version") ||
	fail "could not reach $remote"
remote_main=$(printf '%s\n' "$refs" | awk '$2 == "refs/heads/main" { print $1 }')
[ -n "$remote_main" ] || fail "$remote has no main branch"
[ "$(git rev-parse HEAD)" = "$remote_main" ] ||
	fail "main is not the same as $remote/main. Run: git pull"
printf '%s\n' "$refs" | grep -q "refs/tags/$version\$" && fail "$version is already released"
printf '%s\n' "$refs" | grep -q "refs/heads/$release_branch\$" &&
	fail "$remote already has $release_branch. Open or merge its pull request, or delete it first"
git rev-parse --quiet --verify "refs/heads/$release_branch" >/dev/null &&
	fail "a local branch $release_branch exists. Delete it with: git branch -D $release_branch"
echo "      main matches $remote/main, and $version is new"

# The repository on GitHub, for links: owner/name, or empty
url=$(git remote get-url "$remote")
repo=$(printf '%s\n' "$url" | sed -n -e 's#^git@github\.com:\(.*\)$#\1#p' -e 's#^https://github\.com/\(.*\)$#\1#p' -e 's#^ssh://git@github\.com/\(.*\)$#\1#p' | sed 's#\.git$##')

# Release notes: the summary given, then what changed since the last release
last=$(git describe --tags --abbrev=0 --match 'v*' HEAD 2>/dev/null || true)
changes=$(scripts/release-notes.sh "$last" HEAD)
notes="${NOTES:-}"
if [ -n "$changes" ]; then
	notes="${notes:+$notes

}$changes"
fi
if [ -n "$last" ] && [ -n "$repo" ]; then
	notes="${notes:+$notes

}Full changelog: https://github.com/$repo/compare/$last...$version"
fi
[ -n "$changes" ] || echo "      note: nothing was merged since ${last:-the start}; the notes hold only what NOTES says"

step 3 "Writing $version to VERSION and the README"
git checkout -q -b "$release_branch"
printf '%s\n' "$version" > VERSION
sed "s/Current release: \*\*[^*]*\*\*/Current release: **$version**/" README.md > README.md.tmp
mv README.md.tmp README.md
grep -q "Current release: \*\*$version\*\*" README.md || {
	git checkout -q -- VERSION README.md
	git checkout -q main
	git branch -q -D "$release_branch"
	fail "README.md has no 'Current release: **...**' line to update"
}

step 4 "Checking the version against the codename rules"
if command -v go >/dev/null 2>&1; then
	if ! go test ./internal/version/ >/dev/null 2>&1; then
		go test ./internal/version/ >&2 || true
		git checkout -q -- VERSION README.md
		git checkout -q main
		git branch -q -D "$release_branch"
		fail "$version does not pass the version test; nothing was committed"
	fi
	echo "      passed"
else
	echo "      skipped: go is not installed (CI will run it)"
fi

step 5 "Committing, and pushing $release_branch to $remote (connects to the remote)"
message=$(mktemp)
trap 'rm -f "$message"' EXIT
printf 'chore: release %s\n\n%s\n' "$version" "$notes" > "$message"
git commit -q -a --cleanup=verbatim -F "$message"
git push -u "$remote" "$release_branch" ||
	fail "the push failed. The branch is committed locally; retry with: git push -u $remote $release_branch"

echo
echo "Release notes:"
printf '%s\n' "$notes" | sed 's/^/  /'
echo
echo "Done. $release_branch is pushed."
if [ -n "$repo" ]; then
	title=$(urlencode "chore: release $version")
	body=$(urlencode "$notes")
	echo "Open the pull request: https://github.com/$repo/compare/main...$release_branch?quick_pull=1&title=$title&body=$body"
else
	echo "Open a pull request from $release_branch into main."
fi
echo "When it is merged: git checkout main && git pull && make release"
