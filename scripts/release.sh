#!/bin/sh
# Tag the release named in VERSION and push the tag, which starts the Release
# workflow (archives, GitHub release, Homebrew tap).
#
# Run on main after the release PR is merged and pulled: make release
#
# It refuses rather than guess when the checkout is not exactly what is on
# origin/main, because a pushed tag is not meant to be moved afterwards.
set -eu

version="${1:?usage: scripts/release.sh <version>}"
remote="${RELEASE_REMOTE:-origin}"

fail() {
	echo "release: $1" >&2
	exit 1
}

branch=$(git rev-parse --abbrev-ref HEAD)
[ "$branch" = "main" ] || fail "you are on '$branch'. Run: git checkout main && git pull"

git diff --quiet HEAD || fail "there are uncommitted changes. Commit or stash them first"

git fetch --quiet "$remote" main || fail "could not reach $remote"
head=$(git rev-parse HEAD)
[ "$head" = "$(git rev-parse FETCH_HEAD)" ] ||
	fail "main is not the same as $remote/main. Run: git pull (and make sure the release PR is merged)"

if [ -n "$(git ls-remote --tags "$remote" "refs/tags/$version")" ]; then
	fail "$version is already released. For a new release, change VERSION in a release PR first"
fi

if tagged=$(git rev-parse --quiet --verify "refs/tags/$version^{commit}"); then
	# Left over from an earlier 'make tag': fine if it is this commit
	[ "$tagged" = "$head" ] ||
		fail "a local tag $version points at another commit. Remove it with: git tag -d $version"
	echo "Tag $version already exists on this commit"
else
	git tag -a "$version" -m "Ringmaster $version"
	echo "Tagged $version"
fi

git push "$remote" "$version"
echo "Pushed $version. The Release workflow is now building it."
