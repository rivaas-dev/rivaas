#!/usr/bin/env bash
# Prepare isolates one ready module and create does not duplicate a PR
# or close the parent issue.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
pr="$root/scripts/contrib/pr.sh"
policy="$root/scripts/contrib/pr-policy.sh"
lib="$root/scripts/contrib/lib.sh"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

git init -q -b main "$TMP/src"
git init -q --bare "$TMP/origin"
cd "$TMP/src"
git config user.email test@example.com
git config user.name test
mkdir -p router app
printf 'module example.com/router\n' > router/go.mod
printf 'module example.com/app\n' > app/go.mod
printf 'base\n' > router/a.go
printf 'base\n' > app/a.go
git add .
git commit -q -m init
git remote add origin "$TMP/origin"
git push -q -u origin main

printf 'router-change\n' > router/a.go
printf 'app-change\n' > app/a.go
printf 'staged\n' > app/note.go
git add app/note.go
before=$(git status --porcelain=v1 -z | cksum)

export RIVAAS_NONINTERACTIVE=1
export RIVAAS_OFFLINE=1
export RIVAAS_SKIP_FETCH=1
export RIVAAS_ISSUE_TITLE="Add configurable timeout"
export RIVAAS_MODULES="router app"
export RIVAAS_GROUP=0
export RIVAAS_DEPS="app:router"
export RIVAAS_PREPARE=ready
export RIVAAS_ACTION=prepare
export RIVAAS_WORKTREE_ROOT="$TMP/wts"
export GIT=git
export JQ=jq
export FIND=find
unset RIVAAS_BRANCH || true

bash "$pr" prepare 123

after=$(git status --porcelain=v1 -z | cksum)
[ "$before" = "$after" ] || fail "prepare changed the source status"
git diff --cached --name-only | grep -qx 'app/note.go' || fail "staged app file was lost"

common=$(git rev-parse --git-common-dir)
case "$common" in
  /*) ;;
  *) common="$PWD/$common" ;;
esac
state="${common}/rivaas/pr-state.json"
[ -f "$state" ] || fail "state file missing"
router_wt=$(jq -r '.issues["123"].plans[] | select(.id == "router") | .worktree' "$state")
app_wt=$(jq -r '.issues["123"].plans[] | select(.id == "app") | .worktree' "$state")
[ -n "$router_wt" ] && [ "$router_wt" != null ] || fail "router worktree was not recorded"
[ -z "$app_wt" ] || fail "app was prepared before router is published"
grep -qx 'router-change' "$router_wt/router/a.go" || fail "router change missing from worktree"
grep -qx 'base' "$router_wt/app/a.go" || fail "app change leaked into the router worktree"
[ ! -e "$router_wt/app/note.go" ] || fail "staged app file leaked into the worktree"
jq -e '.issues["123"].plans[] | select(.id == "app") | .depends_on == ["router"]' "$state" >/dev/null || fail "app dependency was not recorded"

# shellcheck source=../lib.sh
source "$lib"
cd "$router_wt"
mapfile -d '' paths < <(list_module_paths router)
printf 'router: add timeout\n' >"$TMP/msg"
commit_paths "$TMP/msg" "${paths[@]}"
cd "$TMP/src"
after_commit=$(git status --porcelain=v1 -z | cksum)
[ "$before" = "$after_commit" ] || fail "commit in the worktree changed the source tree"

cat >"$TMP/gh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${GH_LOG}"
cmd=$1
sub=${2:-}
head=""
body=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--head" ]; then
    head=$(printf '%s' "$arg" | tr '/' '_')
  fi
  if [ "$prev" = "--body-file" ]; then
    body=$arg
  fi
  prev=$arg
done
case "$cmd" in
  auth) exit 0 ;;
  pr)
    case "$sub" in
      list)
        if [ -n "$head" ] && [ -f "${GH_STATE}/$head" ]; then
          cat "${GH_STATE}/$head"
        else
          printf '%s\n' '[]'
        fi
        ;;
      create)
        mkdir -p "${GH_STATE}"
        if [ -n "$body" ]; then
          cp "$body" "${GH_BODY}"
        fi
        printf '%s\n' '[{"number":9,"url":"https://example.test/pull/9","state":"OPEN","isDraft":false}]' > "${GH_STATE}/$head"
        if printf '%s\n' "$*" | grep -q -- '--draft'; then
          printf '%s\n' 'https://example.test/pull/10'
        else
          printf '%s\n' 'https://example.test/pull/9'
        fi
        ;;
      view)
        printf '%s\n' '{"number":9,"url":"https://example.test/pull/9","state":"OPEN","isDraft":false}'
        ;;
      *) exit 1 ;;
    esac
    ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$TMP/gh"

export GH="$TMP/gh"
export GH_LOG="$TMP/gh.log"
export GH_BODY="$TMP/body.md"
export GH_STATE="$TMP/gh-state"
export RIVAAS_PUSH=1
export RIVAAS_SUBISSUE=0
export RIVAAS_DRAFT=auto
: >"$GH_LOG"
cd "$router_wt"
bash "$pr" create
grep -q 'Refs #123' "$GH_BODY" || fail "body does not reference the parent issue"
if grep -Eq 'Fixes #123|Closes #123' "$GH_BODY"; then
  fail "body would close the parent issue"
fi
if grep -q -- '--draft' "$GH_LOG"; then
  fail "router PR should be ready for review"
fi
files=$(mktemp)
printf 'router/a.go\n' >"$files"
bash "$policy" --offline --repo-root "$TMP/src" --title "$(
  jq -r '.issues["123"].plans[] | select(.id == "router") | .branch' "$state" >/dev/null
  printf '%s\n' 'feat(router): add-configurable-timeout'
)" --body-file "$GH_BODY" --files-file "$files" >/dev/null

bash "$pr" create
creates=$(grep -c 'pr create' "$GH_LOG" || true)
[ "$creates" -eq 1 ] || fail "create ran twice: $creates"

# App still depends on an unpublished router, so its PR is a draft.
app_branch=$(jq -r '.issues["123"].plans[] | select(.id == "app") | .branch' "$state")
git -C "$TMP/src" worktree add -q -b "$app_branch" "$TMP/appwt" main
cp "$TMP/src/app/a.go" "$TMP/appwt/app/a.go"
git -C "$TMP/appwt" add app/a.go
git -C "$TMP/appwt" commit -q -m 'app: use the new timeout'
: >"$GH_LOG"
rm -f "$GH_BODY"
cd "$TMP/appwt"
RIVAAS_DRAFT=no bash "$pr" create
grep -q -- '--draft' "$GH_LOG" || fail "dependent PR was not forced to draft"
grep -q 'Refs #123' "$GH_BODY" || fail "draft body dropped the parent reference"
if grep -Eq 'Fixes #123|Closes #123' "$GH_BODY"; then
  fail "draft body would close the parent issue"
fi

# Status must not claim the parent is finished.
status_out=$(bash "$pr" status 123 2>&1 || true)
printf '%s\n' "$status_out" | grep -q 'stays open' || fail "status should keep the parent open: $status_out"

printf 'prepare ok\n'
