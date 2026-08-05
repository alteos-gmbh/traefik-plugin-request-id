#!/usr/bin/env bash
#
# Decides whether the current commit should be released and under which version.
#
# Prints key=value lines on stdout, suitable for appending to $GITHUB_OUTPUT:
#
#   version=v1.2.0     (empty when nothing should be released)
#   release=true|false
#   reason=<human readable explanation>
#
# The version comes from the most recent v* tag, bumped according to
# Conventional Commits found in the commits since it:
#
#   <type>!: ... or a BREAKING CHANGE: body  ->  major
#   feat: ...                                ->  minor
#   anything else                            ->  patch
#
# Releases are skipped when nothing outside documentation, CI, tests and the
# licence changed, so consumers pinning tags do not get a new version for a
# typo fix. Set FORCE_VERSION to release a specific version regardless.
#
# Requires the full history and tags, so check out with fetch-depth: 0.
set -euo pipefail

# Paths that do not affect what Traefik loads at runtime. Tests are included:
# they ship in the source archive but Yaegi ignores _test.go files.
readonly NON_SHIPPING='(\.md$|^\.github/|^Makefile$|_test\.go$|^LICENSE$)'

emit() {
	printf '%s\n' "$@"
}

if [ -n "${FORCE_VERSION:-}" ]; then
	emit "version=${FORCE_VERSION}" "release=true" "reason=version forced through workflow input"
	exit 0
fi

latest=$(git tag --list 'v*' --sort=-v:refname | head -n1 || true)

if [ -z "$latest" ]; then
	emit "version=v1.0.0" "release=true" "reason=no existing tag, so this is the first release"
	exit 0
fi

range="${latest}..HEAD"

if [ -z "$(git log --oneline "$range")" ]; then
	emit "version=" "release=false" "reason=no commits since ${latest}"
	exit 0
fi

if git log -1 --format='%B' | grep -qiE '\[skip release\]'; then
	emit "version=" "release=false" "reason=head commit asks to skip the release"
	exit 0
fi

changed=$(git diff --name-only "$range")
shipping=$(printf '%s\n' "$changed" | grep -vE "$NON_SHIPPING" || true)

if [ -z "$shipping" ]; then
	emit "version=" "release=false" "reason=only docs, CI, tests or the licence changed since ${latest}"
	exit 0
fi

subjects=$(git log --format='%s' "$range")
bodies=$(git log --format='%B' "$range")

if printf '%s\n' "$subjects" | grep -qE '^[a-zA-Z]+(\([^)]*\))?!:' ||
	printf '%s\n' "$bodies" | grep -qE '^BREAKING[ -]CHANGE'; then
	bump="major"
elif printf '%s\n' "$subjects" | grep -qE '^feat(\([^)]*\))?:'; then
	bump="minor"
else
	bump="patch"
fi

current=${latest#v}
major=${current%%.*}
rest=${current#*.}
minor=${rest%%.*}
patch=${rest#*.}
patch=${patch%%-*}

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
patch)
	patch=$((patch + 1))
	;;
esac

next="v${major}.${minor}.${patch}"

if git rev-parse -q --verify "refs/tags/${next}" >/dev/null; then
	emit "version=" "release=false" "reason=${next} already exists"
	exit 0
fi

emit "version=${next}" "release=true" "reason=${bump} bump from ${latest}"
