#!/usr/bin/env bash
# update-actions.sh - pin every third-party GitHub Action this repo uses to its
# latest stable release.
#
# For each `uses: owner/repo[/path]@ref` in .github/workflows/*.yml (and any
# composite actions under .github/actions/), ask the GitHub API for the
# repository's latest release (the /releases/latest endpoint never returns
# drafts or prereleases), resolve that release's tag to a commit, and rewrite
# the reference as
#
#     uses: owner/repo@<full sha> # <tag>
#
# A full SHA is what GitHub's hardening guide recommends: a tag can be moved
# to point at different code (tj-actions/changed-files, March 2025), a commit
# cannot. The trailing comment keeps the version readable, and it is the
# format Dependabot understands, so it can keep bumping these pins later.
#
# Usage:
#   scripts/update-actions.sh             rewrite workflow files in place
#   scripts/update-actions.sh --check     report stale pins; exit 1 if any
#   scripts/update-actions.sh --pin tag   write @v1.2.3 instead of a SHA
#
# Requires an authenticated `gh` (or GH_TOKEN in the environment).

set -euo pipefail

pin=sha
check=0
while (($#)); do
	case $1 in
	--check) check=1 ;;
	--pin)
		shift
		pin=${1:-}
		;;
	--pin=*) pin=${1#--pin=} ;;
	-h | --help)
		sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "update-actions: unknown argument: $1" >&2
		exit 2
		;;
	esac
	shift
done
case $pin in sha | tag) ;; *)
	echo "update-actions: --pin must be 'sha' or 'tag', got '$pin'" >&2
	exit 2
	;;
esac

command -v gh >/dev/null || {
	echo "update-actions: gh is not installed" >&2
	exit 1
}
gh auth status >/dev/null 2>&1 || {
	echo "update-actions: gh is not authenticated (run 'gh auth login' or set GH_TOKEN)" >&2
	exit 1
}

cd "$(dirname "${BASH_SOURCE[0]}")/.."

shopt -s nullglob
files=(.github/workflows/*.yml .github/workflows/*.yaml
	.github/actions/*/action.yml .github/actions/*/action.yaml)
shopt -u nullglob
((${#files[@]})) || {
	echo "update-actions: no workflow files found under .github/" >&2
	exit 1
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cache="$work/resolved" # lines of: owner/repo tag sha
: >"$cache"

# Highest plain version tag (v1, v1.2, v1.2.3) for repositories that tag
# releases without publishing GitHub Releases.
latest_version_tag() {
	gh api --paginate "repos/$1/tags" --jq '.[].name' |
		grep -E '^v?[0-9]+(\.[0-9]+){0,2}$' |
		awk '{ n = $0; sub(/^v/, "", n); split(n, p, ".")
		       printf "%d %d %d %s\n", p[1], p[2], p[3], $0 }' |
		sort -k1,1n -k2,2n -k3,3n | tail -n1 | awk '{ print $4 }'
}

# resolve owner/repo -> sets TAG and SHA (cached per repository).
resolve() {
	local repo=$1 hit
	hit=$(awk -v r="$repo" '$1 == r { print; exit }' "$cache")
	if [[ -n $hit ]]; then
		read -r _ TAG SHA <<<"$hit"
		return
	fi
	TAG=$(gh api "repos/$repo/releases/latest" --jq .tag_name 2>/dev/null) ||
		TAG=$(latest_version_tag "$repo")
	[[ -n $TAG ]] || {
		echo "update-actions: no stable release or version tag found for $repo" >&2
		exit 1
	}
	# /commits/{ref} accepts a tag name and dereferences annotated tags.
	SHA=$(gh api "repos/$repo/commits/$TAG" --jq .sha)
	printf '%s %s %s\n' "$repo" "$TAG" "$SHA" >>"$cache"
}

#            1: indent + "uses:"              2: quote  3: owner/repo
uses_re="^([[:space:]]*-?[[:space:]]*uses:[[:space:]]*)([\"']?)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)"
#            4: optional /sub/path                5: ref                  6: quote  7: trailing comment
uses_re+="((/[^@[:space:]\"']+)?)@([^#[:space:]\"']+)([\"']?)([[:space:]]*#.*)?[[:space:]]*\$"
# A trailing comment that starts with a bare version, e.g. "# v7.0.1", was
# written by a previous run (or by Dependabot) and is replaced. Any further
# comment after it (group 1) and any other comment is preserved.
version_comment_re='^[[:space:]]*#[[:space:]]*v?[0-9]+(\.[0-9]+)*([[:space:]]*#.*)?[[:space:]]*$'

exec 3>&1 # the loop below redirects stdout into the rewritten file
stale=0
for f in "${files[@]}"; do
	out="$work/out"
	changed=0
	while IFS= read -r line || [[ -n $line ]]; do
		if [[ $line =~ $uses_re ]]; then
			prefix=${BASH_REMATCH[1]} oq=${BASH_REMATCH[2]} repo=${BASH_REMATCH[3]}
			sub=${BASH_REMATCH[4]} ref=${BASH_REMATCH[6]} cq=${BASH_REMATCH[7]}
			comment=${BASH_REMATCH[8]}
			[[ $comment =~ $version_comment_re ]] && comment=${BASH_REMATCH[2]}
			resolve "$repo"
			case $pin in
			sha)
				new="${prefix}${oq}${repo}${sub}@${SHA}${cq} # ${TAG}${comment}"
				shown="${SHA:0:12} (${TAG})"
				;;
			tag)
				new="${prefix}${oq}${repo}${sub}@${TAG}${cq}${comment}"
				shown=$TAG
				;;
			esac
			if [[ $new != "$line" ]]; then
				changed=1 stale=1
				printf '%s: %s %s -> %s\n' "$f" "$repo$sub" "$ref" "$shown" >&3
			fi
			line=$new
		fi
		printf '%s\n' "$line"
	done <"$f" >"$out"
	if ((changed && !check)); then
		cp "$out" "$f"
	fi
done

if ((check)); then
	((stale)) && exit 1
	echo "update-actions: all actions are pinned to their latest stable release"
elif ((!stale)); then
	echo "update-actions: nothing to update"
fi
