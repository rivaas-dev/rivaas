#!/usr/bin/env bash
# Module commits must not include staged files from other modules.
set -euo pipefail

lib=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/lib.sh
# shellcheck source=../lib.sh
source "$lib"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local want="$1"
  local got="$2"
  local msg="$3"
  [ "$want" = "$got" ] || fail "$msg: want [$want] got [$got]"
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"
git init -q -b main
git config user.email test@example.com
git config user.name test

mkdir -p router app "router/sub dir" router/benchmarks
printf 'module example.com/router\n' > router/go.mod
printf 'module example.com/app\n' > app/go.mod
printf 'module example.com/bench\n' > router/benchmarks/go.mod
printf 'base\n' > router/a.go
printf 'base\n' > app/a.go
printf 'bench\n' > router/benchmarks/b.go
printf 'gone\n' > router/old.go
git add .
git commit -q -m init

# Partial hunk stays staged in app while router is committed.
printf 'staged\n' > app/a.go
git add app/a.go
printf 'staged\nunstaged\n' > app/a.go
printf 'extra\n' > app/extra.go
git add app/extra.go

printf 'changed\n' > router/a.go
printf 'new\n' > router/new.go
printf 'bin\0data' > "router/sub dir/f.bin"
printf 'bench changed\n' > router/benchmarks/b.go
rm -f router/old.go

# Nested benchmark changes are not part of the router module.
mapfile -d '' router_paths < <(list_module_paths router)
got=$(printf '%s\n' "${router_paths[@]}" | LC_ALL=C sort)
want=$(printf '%s\n' router/a.go router/new.go "router/sub dir/f.bin" router/old.go | LC_ALL=C sort)
assert_eq "$want" "$got" "router path list"

msg=$(mktemp)
printf 'router: isolate the commit\n' >"$msg"
commit_paths "$msg" "${router_paths[@]}"

committed=$(git show --name-only --pretty=format: HEAD | sed '/^$/d' | LC_ALL=C sort)
assert_eq "$want" "$committed" "commit contents"

status=$(git status --porcelain)
printf '%s\n' "$status" | grep -q 'MM app/a.go' || fail "partial app staging was lost: $status"
printf '%s\n' "$status" | grep -q 'A  app/extra.go' || fail "staged app file was lost: $status"
printf '%s\n' "$status" | grep -q 'router/benchmarks/b.go' || fail "benchmark change disappeared: $status"
git show :app/a.go | grep -qx 'staged' || fail "index blob for app/a.go changed"
cached=$(git diff --cached --name-only | LC_ALL=C sort)
printf '%s\n' "$cached" | grep -qx 'app/a.go' || fail "app/a.go no longer staged"
printf '%s\n' "$cached" | grep -qx 'app/extra.go' || fail "app/extra.go no longer staged"
printf '%s\n' "$cached" | grep -q 'router/' && fail "router paths remained staged: $cached"

# A filename with a space round-trips through the commit.
git show HEAD:"router/sub dir/f.bin" | cmp -s - "router/sub dir/f.bin" || fail "binary file mismatch"

printf 'commit safety ok\n'
