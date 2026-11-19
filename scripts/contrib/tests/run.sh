#!/usr/bin/env bash
# Run contribution-workflow shell tests.
set -uo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
failed=0
ran=0

for test in "$root"/*_test.sh; do
  [ -f "$test" ] || continue
  ran=$((ran + 1))
  name=$(basename "$test")
  printf '== %s ==\n' "$name"
  if bash "$test"; then
    printf 'ok  %s\n' "$name"
  else
    printf 'FAIL %s\n' "$name" >&2
    failed=$((failed + 1))
  fi
done

if [ "$ran" -eq 0 ]; then
  printf 'no tests found\n' >&2
  exit 1
fi

if [ "$failed" -ne 0 ]; then
  printf '%s of %s test files failed\n' "$failed" "$ran" >&2
  exit 1
fi

printf '%s test files passed\n' "$ran"
