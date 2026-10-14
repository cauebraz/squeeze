#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    cat <<'USAGE'
usage: scripts/check-commits.sh [base]

Checks the commits in base..HEAD and the branch name against CONTRIBUTING.md:
Conventional Commits subjects, no merge commits, a BREAKING CHANGE footer for
every "!" subject and the reverse, body lines of 72 characters or fewer, and
the <user>/<type>-<description> branch shape. Prints the commit count and the
size of the change outside testdata. base defaults to origin/main. BRANCH
overrides the branch name; CI sets GITHUB_HEAD_REF.
USAGE
    exit 0
fi

base="${1:-origin/main}"
branch="${BRANCH:-${GITHUB_HEAD_REF:-$(git branch --show-current)}}"
types='feat|fix|chore|docs|refactor|test|ci|build|perf|revert'
subject_re="^(${types})(\([a-z0-9-]+\))?(!)?: [a-z0-9\`].*[^. ]$"
branch_re="^[a-z0-9][a-z0-9-]*/(${types})-[a-z0-9]+(-[a-z0-9]+)*$"
max_subject=72
max_body=72
errors=0

fail() {
    echo "$1" >&2
    errors=$((errors + 1))
}

if ! git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
    echo "base $base is not a commit; fetch it or pass another base" >&2
    exit 64
fi

if [ -n "$branch" ] && [ "$branch" != "main" ] && ! [[ "$branch" =~ $branch_re ]]; then
    fail "branch $branch: want <user>/<type>-<description>, for example cauebraz/fix-pytest-fold"
fi

for c in $(git rev-list --merges "$base..HEAD"); do
    fail "$(git log -1 --format='%h %s' "$c"): merge commit; rebase the branch on $base instead"
done

for c in $(git rev-list --no-merges --reverse "$base..HEAD"); do
    short="$(git rev-parse --short "$c")"
    subject="$(git log -1 --format=%s "$c")"
    message="$(git log -1 --format=%B "$c")"
    if ! [[ "$subject" =~ $subject_re ]]; then
        fail "$short $subject: want <type>(<scope>): <summary>, type one of ${types//|/ }, lowercase summary, no trailing period"
    fi
    if [ "${#subject}" -gt "$max_subject" ]; then
        fail "$short $subject: subject has ${#subject} characters, limit $max_subject"
    fi
    if [ "$(printf '%s\n' "$message" | sed -n 2p)" != "" ]; then
        fail "$short $subject: leave line 2 blank between subject and body"
    fi
    bang=0
    footer=0
    [[ "$subject" =~ $subject_re ]] && [ "${BASH_REMATCH[3]}" = "!" ] && bang=1
    printf '%s\n' "$message" | grep -Eq '^BREAKING[ -]CHANGE: .+' && footer=1
    if [ "$bang" -ne "$footer" ]; then
        fail "$short $subject: a breaking change has both \"!\" before the colon and a BREAKING CHANGE: footer"
    fi
    while IFS= read -r line; do
        if [ "${#line}" -gt "$max_body" ] && [[ "$line" != *://* ]]; then
            fail "$short $subject: body line has ${#line} characters, limit $max_body: $line"
        fi
    done < <(printf '%s\n' "$message" | sed 1,2d)
done

commits="$(git rev-list --count "$base..HEAD")"
size="$(git diff --shortstat "$base...HEAD" -- . ':(exclude)**/testdata/**')"
echo "$commits commits on ${branch:-HEAD} since $base;${size:- no changes} outside testdata"

if [ "$errors" -gt 0 ]; then
    echo "$errors problems; CONTRIBUTING.md lists the rules" >&2
    exit 1
fi
