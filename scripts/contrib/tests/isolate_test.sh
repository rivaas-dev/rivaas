#!/usr/bin/env bash
# Isolating one module leaves the source tree untouched and copies binaries,
# deletions, and names that contain spaces.
set -euo pipefail

lib=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/lib.sh
# shellcheck source=../lib.sh
source "$lib"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cd "$TMP/.."
git init -q -b main "$TMP/src"
cd "$TMP/src"
git config user.email test@example.com
git config user.name test

mkdir -p router app "router/sub dir"
printf 'module example.com/router\n' > router/go.mod
printf 'module example.com/app\n' > app/go.mod
printf 'base router\n' > router/a.go
printf 'base app\n' > app/a.go
printf 'keep\n' > router/drop.go
git add .
git commit -q -m init

printf 'router work\n' > router/a.go
printf 'app work\n' > app/a.go
printf 'bin\0zz' > "router/sub dir/f.bin"
rm -f router/drop.go
# Staged unrelated file must remain staged after isolation.
printf 'staged app\n' > app/note.go
git add app/note.go

before=$(git status --porcelain=v1 -z | cksum)

dest="$TMP/wt"
git worktree add -b feat/router "$dest" HEAD >/dev/null

if ! selection_is_safe router; then
  fail "router selection should be safe"
fi
isolate_units "$dest" router

after=$(git status --porcelain=v1 -z | cksum)
[ "$before" = "$after" ] || fail "source status changed"

[ ! -e "$dest/app/note.go" ] || fail "app file leaked into the worktree"
cmp -s app/a.go "$dest/app/a.go" && fail "app worktree should still be the base file"
grep -qx 'base app' "$dest/app/a.go" || fail "dest app file is not the base version"
grep -qx 'router work' "$dest/router/a.go" || fail "router change was not copied"
cmp -s "router/sub dir/f.bin" "$dest/router/sub dir/f.bin" || fail "binary copy mismatch"
[ ! -e "$dest/router/drop.go" ] || fail "deletion was not applied"

# Cross-module content move is refused.
printf 'moved\n' > router/move.go
git add router/move.go
git commit -q -m 'add move'
mkdir -p app
mv router/move.go app/move.go

if selection_is_safe router; then
  fail "cross-module move should be unsafe"
fi
if selection_is_safe router app; then
  :
else
  fail "selecting both sides of a move should be safe"
fi

printf 'isolate ok\n'
