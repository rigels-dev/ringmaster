#!/bin/sh
# Tag the release named in VERSION and push the tag, which starts the Release
# workflow (archives, GitHub release, Homebrew tap).
#
# Run on main after the release PR is merged and pulled: make release
#
# It refuses rather than guess when the checkout is not exactly what is on
# origin/main, because a pushed tag is not meant to be moved afterwards.
#
# It contacts the remote twice: once to compare, once to push. With an SSH key
# that has a passphrase and no ssh-agent, each contact asks for the
# passphrase, so every step is announced before it runs.
set -eu

version="${1:?usage: scripts/release.sh <version>}"
remote="${RELEASE_REMOTE:-origin}"

step() {
	echo "[$1/4] $2"
}

fail() {
	echo "release: $1" >&2
	exit 1
}

step 1 "Checking the local checkout"
branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || fail "you are on '$branch'. Run: git checkout main && git pull"
git diff --quiet HEAD || fail "there are uncommitted changes. Commit or stash them first"
head=$(git rev-parse HEAD)
echo "      on main at $(git rev-parse --short HEAD), nothing uncommitted"

step 2 "Asking $remote for its main and for an existing $version tag (connects to the remote)"
# One connection answers both questions
refs=$(git ls-remote "$remote" refs/heads/main "refs/tags/$version") || fail "could not reach $remote"
remote_main=$(printf '%s\n' "$refs" | awk '$2 == "refs/heads/main" { print $1 }')
remote_tag=$(printf '%s\n' "$refs" | awk -v ref="refs/tags/$version" '$2 == ref { print $1 }')

[ -n "$remote_main" ] || fail "$remote has no main branch"
[ "$head" = "$remote_main" ] ||
	fail "main is not the same as $remote/main. Run: git pull (and make sure the release PR is merged)"
[ -z "$remote_tag" ] ||
	fail "$version is already released. For a new release, change VERSION in a release PR first"
echo "      main matches $remote/main, and $version is not released yet"

step 3 "Tagging this commit as $version"
if tagged=$(git rev-parse --quiet --verify "refs/tags/$version^{commit}"); then
	# Left over from an earlier 'make tag': fine if it is this commit
	[ "$tagged" = "$head" ] ||
		fail "a local tag $version points at another commit. Remove it with: git tag -d $version"
	echo "      the tag already exists on this commit"
else
	git tag -a "$version" -m "Ringmaster $version"
	echo "      tagged"
fi

step 4 "Pushing the tag $version to $remote (connects to the remote)"
git push "$remote" "$version"
echo "Done. $version is pushed, and the Release workflow is now building it."
