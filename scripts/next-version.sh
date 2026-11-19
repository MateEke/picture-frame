#!/usr/bin/env bash
# Print the release tag for the current commit: the stable tag already on HEAD
# when there is one, otherwise the next semver tag derived from conventional
# commits since the last stable release:
#   BREAKING CHANGE / type!:  major
#   feat:                     minor
#   anything else:            patch
# Pre-release and non-semver tags are ignored. Needs a full clone with tags
# (actions/checkout fetch-depth: 0).
set -euo pipefail

stable_tag_re='^v[0-9]+\.[0-9]+\.[0-9]+$'

# A release tag already on HEAD means this commit is done (manual tag or a
# workflow rerun): reuse it so the caller can still dispatch the release.
existing="$(git tag --points-at HEAD | grep -E "$stable_tag_re" | head -n1 || true)"
if [ -n "$existing" ]; then
	echo "$existing"
	exit 0
fi

last="$(git tag --list 'v[0-9]*' --sort=-v:refname | grep -E "$stable_tag_re" | head -n1 || true)"

if [ -n "$last" ]; then
	range="$last..HEAD"
	version="${last#v}"
	IFS=. read -r major minor patch <<<"$version"
else
	range=HEAD
	major=0 minor=0 patch=0
fi

subjects="$(git log "$range" --format=%s)"
bump="patch"
if git log "$range" --format=%B | grep -q 'BREAKING CHANGE' ||
	grep -qE '^[a-z]+(\([^)]*\))?!:' <<<"$subjects"; then
	bump="major"
elif grep -qE '^feat(\([^)]*\))?:' <<<"$subjects"; then
	bump="minor"
fi

case "$bump" in
major)
	major=$((major + 1))
	minor=0
	patch=0
	;;
minor)
	minor=$((minor + 1))
	patch=0
	;;
patch) patch=$((patch + 1)) ;;
esac

echo "v$major.$minor.$patch"
