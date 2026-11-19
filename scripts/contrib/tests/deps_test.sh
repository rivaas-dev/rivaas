#!/usr/bin/env bash
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
cd "$TMP"
mkdir -p router binding app
cat > router/go.mod <<'EOF'
module rivaas.dev/router

go 1.25.0
EOF
cat > binding/go.mod <<'EOF'
module rivaas.dev/binding

go 1.25.0
EOF
cat > app/go.mod <<'EOF'
module rivaas.dev/app

go 1.27.0

require (
	rivaas.dev/binding v0.9.0
	rivaas.dev/router v0.15.1
)
EOF

nodes=$(mktemp)
edges=$(mktemp)
printf '%s\n' router binding app >"$nodes"
printf 'app\trouter\napp\tbinding\n' >"$edges"

ready=$(ready_nodes "$nodes" "$edges" | tr '\n' ' ')
[ "$ready" = "router binding " ] || fail "ready nodes: [$ready]"

order=$(topo_sort "$nodes" "$edges" | tr '\n' ' ')
[ "$order" = "router binding app " ] || fail "order: [$order]"

printf 'app\trouter\nrouter\tapp\n' >"$edges"
if topo_sort "$nodes" "$edges" >/dev/null 2>&1; then
  fail "cycle should fail"
fi

got=$(declared_dep_dirs app | LC_ALL=C sort | tr '\n' ' ')
[ "$got" = "binding router " ] || fail "declared deps: [$got]"
[ -z "$(declared_dep_dirs router)" ] || fail "router should have no internal deps"

slug=$(slugify "Add Configurable Timeout!!!")
[ "$slug" = "add-configurable-timeout" ] || fail "slug: [$slug]"

printf 'deps ok\n'
