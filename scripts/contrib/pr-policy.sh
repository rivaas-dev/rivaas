#!/usr/bin/env bash
# Validate pull request title and body. Warnings do not fail the check.
# GitHub Actions passes the event payload. Tests pass files.
set -uo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib.sh
source "$script_dir/lib.sh"

GH="${GH:-gh}"

usage() {
  cat <<'EOF'
Usage: pr-policy.sh --title TITLE --body-file FILE --files-file FILE [options]

Options:
  --draft true|false     Pull request is a draft (default false)
  --repo-root DIR        Repository root used to discover module scopes
  --dep NUMBER=STATE     Dependency state (OPEN, DRAFT, MERGED, CLOSED, UNKNOWN)
  --offline              Do not call gh

STATE examples: OPEN, DRAFT, MERGED, CLOSED, UNKNOWN
EOF
}

TITLE=""
BODY_FILE=""
FILES_FILE=""
DRAFT=false
REPO_ROOT="."
OFFLINE=0
declare -A DEP_STATE=()

while [ "$#" -gt 0 ]; do
  case "$1" in
    --title) TITLE="${2:-}"; shift 2 ;;
    --body-file) BODY_FILE="${2:-}"; shift 2 ;;
    --files-file) FILES_FILE="${2:-}"; shift 2 ;;
    --draft) DRAFT="${2:-false}"; shift 2 ;;
    --repo-root) REPO_ROOT="${2:-.}"; shift 2 ;;
    --offline) OFFLINE=1; shift ;;
    --dep)
      pair="${2:-}"
      DEP_STATE["${pair%%=*}"]="${pair#*=}"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ -z "$TITLE" ] || [ -z "$BODY_FILE" ] || [ -z "$FILES_FILE" ]; then
  usage >&2
  exit 2
fi
if [ ! -f "$BODY_FILE" ] || [ ! -f "$FILES_FILE" ]; then
  printf 'body file and files list must exist\n' >&2
  exit 2
fi

errors=0
warnings=0

policy_error() {
  errors=$((errors + 1))
  printf '::error::%s\n' "$*"
  printf 'error: %s\n' "$*" >&2
}

policy_warn() {
  warnings=$((warnings + 1))
  printf '::warning::%s\n' "$*"
  printf 'warning: %s\n' "$*" >&2
}

cd "$REPO_ROOT" || {
  printf 'repo root not found: %s\n' "$REPO_ROOT" >&2
  exit 2
}

known_scope() {
  local scope="$1"
  if is_repo_scope "$scope"; then
    return 0
  fi
  [ -f "$scope/go.mod" ]
}

strip_html_comments() {
  awk '
    BEGIN { comment = 0 }
    {
      line = $0
      out = ""
      while (length(line) > 0) {
        if (comment) {
          i = index(line, "-->")
          if (i == 0) {
            line = ""
            break
          }
          line = substr(line, i + 3)
          comment = 0
        } else {
          i = index(line, "<!--")
          if (i == 0) {
            out = out line
            line = ""
            break
          }
          out = out substr(line, 1, i - 1)
          line = substr(line, i + 4)
          comment = 1
        }
      }
      if (!comment) print out
    }
  '
}

summary_text() {
  awk '
    /^# Summary[[:space:]]*$/ { grab = 1; next }
    grab && /^## / { exit }
    grab { print }
  ' "$BODY_FILE" | strip_html_comments
}

section_text() {
  local heading="$1"
  awk -v heading="$heading" '
    $0 == "## " heading { grab = 1; next }
    grab && /^## / { exit }
    grab { print }
  ' "$BODY_FILE"
}

field_value() {
  local label="$1"
  sed -n "s/^\*\*${label}:\*\*[[:space:]]*//p" "$BODY_FILE" | head -1
}

TYPES='feat|fix|refactor|perf|docs|test|build|ci|chore|style|revert'
title_re="^(${TYPES})(\\([A-Za-z0-9._/-]+\\))?: [^[:space:]].{2,}$"
title_scope=""

if ! printf '%s\n' "$TITLE" | grep -Eq "$title_re"; then
  policy_error "PR title must look like \"feat(router): add configurable timeout\" or \"docs: clarify contribution workflow\". Types: feat, fix, refactor, perf, docs, test, build, ci, chore, style, revert."
else
  title_scope=$(printf '%s\n' "$TITLE" | sed -n 's/^[^(]*(\([^)]*\)).*/\1/p')
  if [ -n "$title_scope" ] && ! known_scope "$title_scope"; then
    policy_error "Scope \"${title_scope}\" is not a module directory or a repository category (ci, nix, dotfiles, deps, docs, middleware, scripts, repo)."
  fi
fi

summary=$(summary_text | tr -d '[:space:]')
if [ "${#summary}" -lt 10 ]; then
  policy_error "Summary is missing. Replace the HTML comment under \"# Summary\" with a short description of the change."
fi

related=$(section_text "Related Issue")
if printf '%s\n' "$related" | grep -Eq '(Refs|Closes|Fixes|Resolves|Relates to) #[0-9]+'; then
  :
elif printf '%s\n' "$related" | grep -Eq '^No related issue \(maintenance\)\.[[:space:]]*$'; then
  :
else
  policy_error "Related Issue must contain \"Refs #<number>\" (or Closes/Fixes) or the line \"No related issue (maintenance).\" An empty \"Refs #\" does not count."
fi

if printf '%s\n' "$related" | grep -Eq '(Fixes|Closes|Resolves) #[0-9]+' \
  && printf '%s\n' "$related" | grep -Eq 'parent issue|Fixes #[0-9]+'; then
  :
fi

primary=$(field_value "Primary module")
primary=$(printf '%s' "$primary" | sed 's/[[:space:]]*$//')
if [ -z "$primary" ]; then
  policy_error "Set **Primary module:** to the module directory (for example router or middleware/cors), a repository category, or repo."
elif ! known_scope "$primary"; then
  policy_error "Primary module \"${primary}\" is not a known module or repository category."
fi

additional=$(field_value "Additional modules")
additional=$(printf '%s' "$additional" | sed 's/[[:space:]]*$//')
additional_norm=$(printf '%s' "$additional" | tr '[:upper:]' '[:lower:]')

declare -A extra=()
if [ -n "$additional" ] && [ "$additional_norm" != none ]; then
  IFS=',' read -r -a extra_items <<<"$additional"
  for item in "${extra_items[@]}"; do
    item=$(printf '%s' "$item" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    [ -z "$item" ] && continue
    if ! known_scope "$item"; then
      policy_warn "Additional module \"${item}\" is not a known module or repository category."
    fi
    extra["$item"]=1
  done
fi

if [ -z "$(field_value "Breaking changes")" ]; then
  policy_warn "Add **Breaking changes:** (No, or a short description) so reviewers can see the release impact."
fi

depends=$(field_value "Depends on")
depends=$(printf '%s' "$depends" | sed 's/[[:space:]]*$//')
depends_norm=$(printf '%s' "$depends" | tr '[:upper:]' '[:lower:]')

lookup_dep() {
  local num="$1"
  if [ -n "${DEP_STATE[$num]:-}" ]; then
    printf '%s\n' "${DEP_STATE[$num]}"
    return 0
  fi
  if [ "$OFFLINE" -eq 1 ]; then
    printf '%s\n' UNKNOWN
    return 0
  fi
  local raw
  if ! raw=$("$GH" pr view "$num" --json state,isDraft --jq 'if .isDraft then "DRAFT" else .state end' 2>/dev/null); then
    printf '%s\n' UNKNOWN
    return 0
  fi
  printf '%s\n' "$raw"
}

if [ -n "$depends" ] && [ "$depends_norm" != none ]; then
  while read -r num; do
    [ -z "$num" ] && continue
    state=$(lookup_dep "$num")
    case "$state" in
      MERGED|CLOSED) ;;
      OPEN|DRAFT)
        policy_warn "Depends on #${num}, which is ${state}. Keep this pull request in draft until that dependency is merged, and do not treat it as independently merge-ready."
        ;;
      *)
        policy_warn "Could not verify dependency #${num}. Check it before marking this pull request ready."
        ;;
    esac
  done < <(printf '%s\n' "$depends" | grep -Eo '#[0-9]+' | tr -d '#' | sort -u)
fi

declare -A modules_hit=()
declare -A scopes_hit=()
while IFS= read -r file; do
  [ -z "$file" ] && continue
  scope=$(category_for_path "$file")
  if [ -z "$scope" ]; then
    policy_warn "Could not assign \"${file}\" to a module. Mention it in the summary if it is intentional."
    continue
  fi
  scopes_hit["$scope"]=1
  if [ -f "$scope/go.mod" ]; then
    modules_hit["$scope"]=1
  fi
  if [ -n "$primary" ] && [ "$scope" != "$primary" ] && [ -z "${extra[$scope]:-}" ]; then
    policy_warn "\"${file}\" is outside the primary module (${primary}). List it under Additional modules or move it to its own pull request."
  fi
done <"$FILES_FILE"

module_count=0
for _m in "${!modules_hit[@]}"; do
  module_count=$((module_count + 1))
done
if [ "$module_count" -gt 1 ] && { [ -z "$additional" ] || [ "$additional_norm" = none ]; }; then
  policy_warn "This pull request changes ${module_count} modules but Additional modules is None. Use one pull request per module unless you explain the grouping."
fi

gomod_changed=0
if grep -Eq '(^|/)go\.mod$' "$FILES_FILE"; then
  gomod_changed=1
fi
if [ "$gomod_changed" -eq 1 ] && { [ -z "$depends" ] || [ "$depends_norm" = none ]; }; then
  policy_warn "A go.mod file changed and Depends on is None. Confirm the required module versions are already published. A local workspace replace does not prove that."
fi

if [ -n "$primary" ] && [ -n "$title_scope" ] && [ "$title_scope" != "$primary" ]; then
  policy_warn "Title scope (${title_scope}) and primary module (${primary}) differ. Use the primary module as the scope unless you mean both."
fi

if [ "$errors" -gt 0 ]; then
  printf 'PR policy failed with %s error(s) and %s warning(s).\n' "$errors" "$warnings" >&2
  exit 1
fi
if [ "$warnings" -gt 0 ]; then
  printf 'PR policy passed with %s warning(s).\n' "$warnings"
else
  printf 'PR policy passed.\n'
fi
exit 0
