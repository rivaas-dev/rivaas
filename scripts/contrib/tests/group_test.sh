#!/usr/bin/env bash
# A justified group copies every selected module into one worktree.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
pr="$root/scripts/contrib/pr.sh"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
git init -q -b main "$TMP/src"
cd "$TMP/src"
git config user.email test@example.com
git config user.name test
mkdir -p binding logging
printf 'module example.com/binding\n' > binding/go.mod
printf 'module example.com/logging\n' > logging/go.mod
printf 'base\n' > binding/a.go
printf 'base\n' > logging/a.go
git add .
git commit -q -m init
printf 'bind\n' > binding/a.go
printf 'log\n' > logging/a.go
before=$(git status --porcelain=v1 -z | cksum)

export RIVAAS_NONINTERACTIVE=1 RIVAAS_OFFLINE=1 RIVAAS_SKIP_FETCH=1
export RIVAAS_ISSUE_TITLE="Share a request id"
export RIVAAS_MODULES="binding logging"
export RIVAAS_GROUP=1
export RIVAAS_JUSTIFICATION="The log field is only useful with the new binding."
export RIVAAS_PREPARE=ready RIVAAS_ACTION=prepare
export RIVAAS_WORKTREE_ROOT="$TMP/wts"
export GIT=git JQ=jq FIND=find
unset RIVAAS_BRANCH || true

bash "$pr" prepare 50
after=$(git status --porcelain=v1 -z | cksum)
[ "$before" = "$after" ] || fail "grouped prepare changed the source tree"

common=$(git rev-parse --git-common-dir)
case "$common" in
  /*) ;;
  *) common="$PWD/$common" ;;
esac
state="${common}/rivaas/pr-state.json"
count=$(jq '.issues["50"].plans | length' "$state")
[ "$count" = 1 ] || fail "group should be one plan, got $count"
mods=$(jq -r '.issues["50"].plans[0].modules | join(",")' "$state")
[ "$mods" = "binding,logging" ] || fail "group modules: $mods"
wt=$(jq -r '.issues["50"].plans[0].worktree' "$state")
grep -qx 'bind' "$wt/binding/a.go" || fail "binding change missing"
grep -qx 'log' "$wt/logging/a.go" || fail "logging change missing"
jq -e '.issues["50"].plans[0].justification | length > 0' "$state" >/dev/null || fail "justification missing"

printf 'group ok\n'
