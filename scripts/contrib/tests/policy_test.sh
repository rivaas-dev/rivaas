#!/usr/bin/env bash
# Representative PR policy cases, including a fork-safe workflow shape.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
policy="$root/scripts/contrib/pr-policy.sh"
repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT

mkdir -p "$repo/router" "$repo/app" "$repo/logging" "$repo/.github/workflows"
printf 'module example.com/router\n' >"$repo/router/go.mod"
printf 'module example.com/app\n' >"$repo/app/go.mod"
printf 'module example.com/logging\n' >"$repo/logging/go.mod"
git -C "$repo" init -q -b main
git -C "$repo" config user.email test@example.com
git -C "$repo" config user.name test

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

run_case() {
  local name="$1"
  local title="$2"
  local body="$3"
  local files="$4"
  local expect="$5"
  shift 5
  local body_file files_file
  body_file=$(mktemp)
  files_file=$(mktemp)
  printf '%s\n' "$body" >"$body_file"
  printf '%s\n' "$files" >"$files_file"
  local out rc
  set +e
  out=$(bash "$policy" --offline --repo-root "$repo" --title "$title" --body-file "$body_file" --files-file "$files_file" "$@" 2>&1)
  rc=$?
  set -e
  rm -f "$body_file" "$files_file"
  if [ "$rc" -ne "$expect" ]; then
    printf '%s\n' "$out" >&2
    fail "$name: exit $rc, want $expect"
  fi
  printf '%s\n' "$out"
}

valid_body() {
  cat <<'EOF'
# Summary

Add a configurable request timeout to the router.

## Related Issue

Refs #123

## Module Scope

**Primary module:** router

**Additional modules:** None

**Breaking changes:** No

## Dependencies

**Depends on:** None

## Testing

go test -C router ./... passed.

## Checklist

- [ ] Relevant tests pass.
EOF
}

out=$(run_case "valid feature" "feat(router): add configurable timeout" "$(valid_body)" "router/timeout.go" 0)
printf '%s\n' "$out" | grep -q 'PR policy passed' || fail "valid feature should pass cleanly"

out=$(run_case "bad title" "add timeout" "$(valid_body)" "router/timeout.go" 1)
printf '%s\n' "$out" | grep -q 'PR title must look like' || fail "bad title message"

body=$(valid_body | awk 'BEGIN{skip=0} /^# Summary/{print; skip=1; next} skip && /^## /{skip=0} skip{next} {print}')
out=$(run_case "missing summary" "feat(router): add configurable timeout" "$body" "router/timeout.go" 1)
printf '%s\n' "$out" | grep -q 'Summary is missing' || fail "missing summary"

body=$(valid_body | sed 's/Refs #123/Refs #/')
out=$(run_case "empty refs" "feat(router): add configurable timeout" "$body" "router/timeout.go" 1)
printf '%s\n' "$out" | grep -q 'Related Issue' || fail "empty refs"

body=$(valid_body | sed 's/Refs #123/No related issue (maintenance)./')
body=$(printf '%s\n' "$body" | sed 's/\*\*Primary module:\*\* router/**Primary module:** docs/')
out=$(run_case "docs maintenance" "docs: clarify contribution workflow" "$body" "CONTRIBUTING.md" 0)
printf '%s\n' "$out" | grep -q 'PR policy passed' || fail "docs maintenance should pass"

body=$(valid_body | sed 's/\*\*Additional modules:\*\* None/**Additional modules:** logging/')
body=$(printf '%s\n' "$body" | sed 's/Add a configurable request timeout to the router./Update the router and the logging helper together because the log field is part of the same timeout behavior./')
out=$(run_case "multi module explained" "feat(router): record timeout decisions" "$body" $'router/timeout.go\nlogging/log.go' 0)
if printf '%s\n' "$out" | grep -q 'outside the primary module'; then
  fail "listed additional module should not be unexpected: $out"
fi
printf '%s\n' "$out" | grep -q 'changes 2 modules' && fail "explained multi-module should not warn about None"

body=$(valid_body)
out=$(run_case "multi module unexplained" "feat(router): record timeout decisions" "$body" $'router/timeout.go\nlogging/log.go' 0)
printf '%s\n' "$out" | grep -q 'changes 2 modules' || fail "unexplained multi-module should warn"
printf '%s\n' "$out" | grep -q '::warning::' || fail "warning annotation missing"

draft_body=$(valid_body | sed 's/\*\*Depends on:\*\* None/**Depends on:** #456/')
out=$(run_case "draft dependency" "feat(app): use router timeout" "$(
  printf '%s\n' "$draft_body" | sed 's/\*\*Primary module:\*\* router/**Primary module:** app/'
)" "app/app.go" 0 --draft true --dep 456=OPEN)
printf '%s\n' "$out" | grep -q 'Depends on #456' || fail "unresolved dependency should warn"
printf '%s\n' "$out" | grep -q 'PR policy passed with' || fail "draft dependency should not fail"

# Fork PRs run the untrusted workflow with read-only permissions, not pull_request_target.
workflow="$root/.github/workflows/pr-policy.yml"
[ -f "$workflow" ] || fail "workflow missing"
if grep -q 'pull_request_target' "$workflow"; then
  fail "workflow must not use pull_request_target"
fi
grep -q 'contents: read' "$workflow" || fail "workflow should be read-only"
grep -q 'pull-requests: read' "$workflow" || fail "workflow should read pull requests"
if grep -Eq 'pull-requests: write|contents: write' "$workflow"; then
  fail "workflow grants write permissions"
fi
grep -q 'edited' "$workflow" || fail "workflow should run when the PR text is edited"

printf 'policy ok\n'
