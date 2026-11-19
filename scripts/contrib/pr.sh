#!/usr/bin/env bash
# Prepare and open module-scoped pull requests.
# Git state comes from git. GitHub state comes from gh. AI text is optional.
set -uo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib.sh
source "$script_dir/lib.sh"

GH="${GH:-gh}"

die() {
  ui_error "$*"
  exit 1
}

need_gum() {
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    return 0
  fi
  if ! ui_gum --version >/dev/null 2>&1; then
    die "gum is required for the interactive prompts. Install gum, or follow the manual workflow in CONTRIBUTING.md."
  fi
}

confirm_default_yes() {
  local prompt="$1"
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    return 0
  fi
  ui_gum confirm "$prompt" </dev/tty
}

confirm_default_no() {
  local prompt="$1"
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    return 1
  fi
  ui_gum confirm "$prompt" </dev/tty
}

prompt_line() {
  local label="$1"
  local value="$2"
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    printf '%s\n' "$value"
    return 0
  fi
  ui_gum input --header "$label" --value "$value" </dev/tty
}

require_gh() {
  if [ ! -x "$GH" ] && ! command -v "$GH" >/dev/null 2>&1; then
    die "The gh command is not available. Install GitHub CLI and run 'gh auth login', or open the pull request in the browser. CONTRIBUTING.md has the manual steps."
  fi
  if ! "$GH" auth status >/dev/null 2>&1; then
    die "gh is not logged in. Run 'gh auth login', then retry. The manual workflow in CONTRIBUTING.md does not need gh."
  fi
}

json_array() {
  if [ "$#" -eq 0 ]; then
    printf '%s\n' '[]'
    return 0
  fi
  "$JQ" -n -c '$ARGS.positional' --args "$@"
}

read_issue() {
  local number="$1"
  if [ "${RIVAAS_OFFLINE:-}" = 1 ]; then
    ISSUE_TITLE=${RIVAAS_ISSUE_TITLE:-"Issue ${number}"}
    ISSUE_STATE=OPEN
    ISSUE_URL=""
    return 0
  fi
  require_gh
  local json
  if ! json=$("$GH" issue view "$number" --json number,title,state,url); then
    die "Could not read issue #${number}. Check the number and 'gh auth status'."
  fi
  ISSUE_TITLE=$(printf '%s' "$json" | "$JQ" -r '.title')
  ISSUE_STATE=$(printf '%s' "$json" | "$JQ" -r '.state')
  ISSUE_URL=$(printf '%s' "$json" | "$JQ" -r '.url')
}

maybe_fetch() {
  if [ "${RIVAAS_SKIP_FETCH:-}" = 1 ] || [ "${RIVAAS_OFFLINE:-}" = 1 ]; then
    return 0
  fi
  if ! "$GIT" remote get-url origin >/dev/null 2>&1; then
    ui_warn "No origin remote. Using local branches only."
    return 0
  fi
  if ! "$GIT" fetch origin --prune >/dev/null 2>&1; then
    ui_warn "Could not fetch origin. Using local git data."
  fi
}

warn_if_linked_worktree() {
  local git_dir common
  git_dir=$(cd "$("$GIT" rev-parse --git-dir)" && pwd)
  common=$(cd "$("$GIT" rev-parse --git-common-dir)" && pwd)
  if [ "$git_dir" != "$common" ]; then
    ui_warn "This directory is a linked worktree. Only changes in this worktree are visible."
  fi
}

modules_with_changes() {
  local unit staged unstaged untracked total
  while IFS= read -r unit; do
    [ -z "$unit" ] && continue
    read -r staged unstaged untracked <<<"$(count_changes "$unit")"
    total=$((staged + unstaged + untracked))
    if [ "$total" -gt 0 ]; then
      printf '%s\n' "$unit"
    fi
  done < <(find_modules)
}

show_change_table() {
  local unit staged unstaged untracked
  printf 'Module\tStaged\tModified\tUntracked\n'
  while IFS= read -r unit; do
    [ -z "$unit" ] && continue
    read -r staged unstaged untracked <<<"$(count_changes "$unit")"
    if is_root_category "$unit"; then
      printf 'root(%s)\t%s\t%s\t%s\n' "$unit" "$staged" "$unstaged" "$untracked"
    else
      printf '%s\t%s\t%s\t%s\n' "$unit" "$staged" "$unstaged" "$untracked"
    fi
  done
}

pick_modules() {
  local available="$1"
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    local want="${RIVAAS_MODULES:-}"
    [ -n "$want" ] || die "Set RIVAAS_MODULES to the modules to prepare."
    local unit
    for unit in $want; do
      if ! printf '%s\n' "$available" | grep -Fxq "$unit"; then
        die "Module ${unit} has no uncommitted changes."
      fi
      printf '%s\n' "$unit"
    done
    return 0
  fi
  ui_info "Modules with uncommitted changes:"
  echo ""
  show_change_table <<<"$available" | ui_gum table --print --separator "$(printf '\t')" || show_change_table <<<"$available"
  echo ""
  ui_info "Space selects a module. Enter confirms. Each module becomes its own pull request unless you group them."
  local picked
  picked=$(printf '%s\n' "$available" | ui_gum choose --no-limit --header "Select modules for this issue:" </dev/tty) || true
  [ -n "$picked" ] || die "No modules selected."
  printf '%s\n' "$picked"
}

ask_group() {
  local count="$1"
  [ "$count" -gt 1 ] || return 1
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    [ "${RIVAAS_GROUP:-0}" = 1 ]
    return
  fi
  local choice
  choice=$(ui_gum choose --header "How should these modules be split?" \
    "Separate pull requests (recommended)" \
    "One pull request for all selected modules" </dev/tty) || true
  [ "$choice" = "One pull request for all selected modules" ]
}

ask_justification() {
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    printf '%s\n' "${RIVAAS_JUSTIFICATION:-}"
    return 0
  fi
  prompt_line "Why do these modules belong in one pull request?" ""
}

# Change-level edges. A declared go.mod requirement is only a hint.
ask_change_deps() {
  local selected="$1"
  local count
  count=$(printf '%s\n' "$selected" | grep -c . || true)
  [ "$count" -gt 1 ] || return 0
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    local raw="${RIVAAS_DEPS:-}"
    local edge
    IFS=',' read -r -a edges <<<"$raw"
    for edge in "${edges[@]+"${edges[@]}"}"; do
      [ -z "$edge" ] && continue
      local dep=${edge%%:*}
      local on=${edge#*:}
      [ -n "$dep" ] && [ -n "$on" ] && [ "$dep" != "$on" ] || continue
      printf '%s\t%s\n' "$dep" "$on"
    done
    return 0
  fi
  echo ""
  ui_info "Declared Go requirements among the selection (not the same as a change-level dependency):"
  local unit deps
  local any=0
  while IFS= read -r unit; do
    [ -z "$unit" ] && continue
    deps=$(declared_dep_dirs "$unit" | while IFS= read -r dep; do
      [ -z "$dep" ] && continue
      printf '%s\n' "$selected" | grep -Fxq "$dep" && printf '%s\n' "$dep"
    done || true)
    if [ -n "$deps" ]; then
      any=1
      while IFS= read -r dep; do
        [ -z "$dep" ] && continue
        printf '  %s requires %s\n' "$unit" "$dep"
      done <<<"$deps"
    fi
  done <<<"$selected"
  [ "$any" -eq 1 ] || ui_info "  None."
  echo ""
  ui_info "Select changes that need new, unpublished functionality from another selected module."
  local -a options=()
  local a b
  while IFS= read -r a; do
    [ -z "$a" ] && continue
    while IFS= read -r b; do
      [ -z "$b" ] && continue
      [ "$a" = "$b" ] && continue
      options+=("$a -> $b")
    done <<<"$selected"
  done <<<"$selected"
  local picked
  picked=$(printf '%s\n' "${options[@]}" | ui_gum choose --no-limit --header "Change-level dependencies:" </dev/tty) || true
  while IFS= read -r edge; do
    [ -z "$edge" ] && continue
    printf '%s\t%s\n' "${edge%% -> *}" "${edge##* -> }"
  done <<<"$picked"
}

branch_for_unit() {
  local unit="$1"
  local issue="$2"
  local title="$3"
  local type slug safe key var generated
  type=${RIVAAS_TYPE:-feat}
  slug=$(slugify "$title")
  [ -n "$slug" ] || slug=change
  safe=$(printf '%s' "$unit" | tr '/' '-')
  generated="${type}/${safe}/${issue}-${slug}"
  key=$(printf '%s' "$unit" | tr '/' '_')
  var="RIVAAS_BRANCH_${key}"
  if [ -n "${!var:-}" ]; then
    printf '%s\n' "${!var}"
    return 0
  fi
  if [ -n "${RIVAAS_BRANCH:-}" ] && [ "${PLAN_COUNT:-1}" = 1 ]; then
    printf '%s\n' "$RIVAAS_BRANCH"
    return 0
  fi
  prompt_line "Branch for ${unit}" "$generated"
}

valid_branch() {
  local branch="$1"
  case "$branch" in
    ''|*' '*|*..*|*~*|*'^'*|*:*|*'?'*|*'['*) return 1 ;;
    /*|*/) return 1 ;;
  esac
  return 0
}

ensure_worktree() {
  local branch="$1"
  local base_sha="$2"
  local dest
  dest=$(worktree_path_for_branch "$branch")
  case "$dest" in
    "$PWD"|"$PWD"/*)
      die "Refusing to create a worktree inside the current working tree (${dest})."
      ;;
  esac
  if "$GIT" worktree list --porcelain | grep -Fxq "worktree ${dest}"; then
    printf '%s\n' "$dest"
    return 0
  fi
  mkdir -p -- "$(dirname -- "$dest")"
  if "$GIT" show-ref --verify --quiet "refs/heads/${branch}"; then
    "$GIT" worktree add "$dest" "$branch" >&2 || die "Could not add a worktree for ${branch}."
  else
    "$GIT" worktree add -b "$branch" "$dest" "$base_sha" >&2 || die "Could not create branch ${branch}."
  fi
  printf '%s\n' "$dest"
}

worktree_commit_count() {
  local dest="$1"
  local base_sha="$2"
  "$GIT" -C "$dest" rev-list --count "${base_sha}..HEAD"
}

upsert_issue() {
  local issue="$1"
  local title="$2"
  local base="$3"
  local file tmp
  file=$(ensure_state_file)
  tmp=$(mktemp)
  "$JQ" --arg issue "$issue" --arg title "$title" --arg base "$base" '
    .issues[$issue] = (.issues[$issue] // {issue: ($issue | tonumber), title: $title, base: $base, plans: []})
    | .issues[$issue].title = $title
    | .issues[$issue].base = $base
  ' "$file" >"$tmp"
  mv "$tmp" "$file"
}

upsert_plan() {
  local issue="$1"
  local plan="$2"
  local file tmp id
  file=$(ensure_state_file)
  id=$(printf '%s' "$plan" | "$JQ" -r '.id')
  tmp=$(mktemp)
  "$JQ" --arg issue "$issue" --arg id "$id" --argjson plan "$plan" '
    .issues[$issue].plans = ((.issues[$issue].plans // []) | map(select(.id != $id))) + [$plan]
  ' "$file" >"$tmp"
  mv "$tmp" "$file"
}

get_plan() {
  local issue="$1"
  local id="$2"
  local file
  file=$(state_file)
  [ -f "$file" ] || return 1
  "$JQ" -e --arg issue "$issue" --arg id "$id" \
    '.issues[$issue].plans[] | select(.id == $id)' "$file"
}

make_plan_json() {
  local id="$1"
  local modules_json="$2"
  local branch="$3"
  local worktree="$4"
  local depends_json="$5"
  local justification="$6"
  local adopted="$7"
  local landed="${8:-}"
  "$JQ" -n \
    --arg id "$id" \
    --argjson modules "$modules_json" \
    --arg branch "$branch" \
    --arg worktree "$worktree" \
    --argjson depends_on "$depends_json" \
    --arg justification "$justification" \
    --argjson adopted "$adopted" \
    --arg landed "$landed" \
    '{
      id: $id,
      modules: $modules,
      branch: $branch,
      worktree: $worktree,
      depends_on: $depends_on,
      justification: $justification,
      sub_issue: null,
      pr_number: null,
      pr_url: null,
      adopted: $adopted,
      landed_commit: (if $landed == "" then null else $landed end),
      prepared_at: (now | todate)
    }'
}

plan_dependency_ready() {
  local issue="$1"
  local dep="$2"
  local override=",${RIVAAS_DEP_STATE:-},"
  case "$override" in
    *",${dep}=ready,"*) return 0 ;;
    *",${dep}=blocked,"*) return 1 ;;
  esac
  local plan pr landed sha state merge_sha
  plan=$(get_plan "$issue" "$dep" 2>/dev/null) || return 1
  landed=$(printf '%s' "$plan" | "$JQ" -r '.landed_commit // empty')
  pr=$(printf '%s' "$plan" | "$JQ" -r '.pr_number // empty')
  sha=""
  if [ -n "$landed" ]; then
    sha=$landed
  elif [ -n "$pr" ] && [ "${RIVAAS_OFFLINE:-}" != 1 ]; then
    if ! state=$("$GH" pr view "$pr" --json state,mergeCommit --jq '[.state, (.mergeCommit.oid // "")] | join(" ")' 2>/dev/null); then
      return 1
    fi
    merge_sha=${state#* }
    state=${state%% *}
    [ "$state" = MERGED ] || return 1
    sha=$merge_sha
  else
    return 1
  fi
  [ -n "$sha" ] || return 1
  if [ -f "${dep}/go.mod" ]; then
    tag_contains_commit "$dep" "$sha"
    return
  fi
  return 0
}

prepare_one_plan() {
  local issue="$1"
  local base_sha="$2"
  local id="$3"
  local branch="$4"
  shift 4
  local -a units=("$@")
  local dest count
  if ! selection_is_safe "${units[@]}"; then
    die "Cannot isolate ${id}. A rename or copy crosses a module boundary that is not part of this pull request."
  fi
  dest=$(ensure_worktree "$branch" "$base_sha")
  count=$(worktree_commit_count "$dest" "$base_sha")
  if [ "$count" -gt 0 ]; then
    ui_info "Branch ${branch} already has ${count} commit(s). Adopting it without copying."
    printf '%s\n' "$dest"
    return 0
  fi
  if [ -n "$("$GIT" -C "$dest" status --porcelain)" ]; then
    die "Worktree ${dest} already has uncommitted changes. Commit or remove that worktree, then run prepare again. The original working tree was not modified."
  fi
  isolate_units "$dest" "${units[@]}" || die "Could not copy ${id} into ${dest}. The original working tree was not modified."
  printf '%s\n' "$dest"
}

prepare_new() {
  local issue="$1"
  local base="$2"
  local base_sha="$3"
  local available selected count group justification
  available=$(modules_with_changes)
  if [ -z "$available" ]; then
    die "No uncommitted module changes. Adopt an existing branch or commit, or change a module first."
  fi
  selected=$(pick_modules "$available")
  count=$(printf '%s\n' "$selected" | grep -c . || true)
  group=0
  justification=""
  if ask_group "$count"; then
    group=1
    justification=$(ask_justification)
    [ -n "$justification" ] || die "A grouped pull request needs a one-line reason."
  fi

  local -a units=()
  local unit
  while IFS= read -r unit; do
    [ -z "$unit" ] && continue
    units+=("$unit")
  done <<<"$selected"

  local edges_file nodes_file
  edges_file=$(mktemp)
  nodes_file=$(mktemp)
  if [ "$group" -eq 1 ]; then
    printf '%s\n' "${units[0]}" >"$nodes_file"
    : >"$edges_file"
  else
    printf '%s\n' "${units[@]}" >"$nodes_file"
    ask_change_deps "$selected" >"$edges_file"
    if ! topo_sort "$nodes_file" "$edges_file" >/dev/null; then
      die "Those change-level dependencies form a cycle. Drop an edge and try again."
    fi
  fi

  upsert_issue "$issue" "$ISSUE_TITLE" "$base"
  PLAN_COUNT=$count

  local -a plan_ids=()
  if [ "$group" -eq 1 ]; then
    plan_ids=("${units[0]}")
  else
    plan_ids=("${units[@]}")
  fi

  local id branch modules_json depends_json dest
  for id in "${plan_ids[@]}"; do
    if [ "$group" -eq 1 ]; then
      modules_json=$(json_array "${units[@]}")
      branch=$(branch_for_unit "$id" "$issue" "$ISSUE_TITLE")
    else
      modules_json=$(json_array "$id")
      branch=$(branch_for_unit "$id" "$issue" "$ISSUE_TITLE")
    fi
    valid_branch "$branch" || die "Branch name is not valid: ${branch}"
    depends_json='[]'
    if [ "$group" -eq 0 ]; then
      local -a deps=()
      local line dep_on
      while IFS= read -r line; do
        [ -z "$line" ] && continue
        [ "${line%%$'\t'*}" = "$id" ] || continue
        dep_on=${line#*$'\t'}
        deps+=("$dep_on")
      done <"$edges_file"
      depends_json=$(json_array ${deps[@]+"${deps[@]}"})
    fi
    local plan
    plan=$(make_plan_json "$id" "$modules_json" "$branch" "" "$depends_json" "$justification" false "")
    upsert_plan "$issue" "$plan"
  done

  local -a ready=()
  if [ "$group" -eq 1 ]; then
    ready=("${plan_ids[0]}")
  else
    while IFS= read -r id; do
      [ -z "$id" ] && continue
      local waiting=0
      local plan dep
      plan=$(get_plan "$issue" "$id")
      while IFS= read -r dep; do
        [ -z "$dep" ] && continue
        if ! plan_dependency_ready "$issue" "$dep"; then
          waiting=1
        fi
      done < <(printf '%s' "$plan" | "$JQ" -r '.depends_on[]?')
      if [ "$waiting" -eq 0 ]; then
        ready+=("$id")
      else
        ui_warn "${id} waits on unpublished dependencies. It was recorded and not copied."
      fi
    done <"$nodes_file"
  fi

  local want="${RIVAAS_PREPARE:-ready}"
  local -a chosen=()
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    if [ "$want" = ready ]; then
      chosen=("${ready[@]+"${ready[@]}"}")
    else
      local item
      for item in $want; do
        local ok=0
        local r
        for r in "${ready[@]+"${ready[@]}"}"; do
          [ "$r" = "$item" ] && ok=1
        done
        [ "$ok" -eq 1 ] || die "${item} is not ready to prepare. Its change-level dependencies are still unpublished."
        chosen+=("$item")
      done
    fi
  else
    if [ "${#ready[@]}" -eq 0 ]; then
      ui_info "Nothing is ready to prepare. Status shows what is waiting."
      return 0
    fi
    ui_info "Ready to prepare now:"
    printf '  %s\n' "${ready[@]}"
    local picked
    picked=$(printf '%s\n' "${ready[@]}" | ui_gum choose --no-limit --header "Prepare these pull requests now:" </dev/tty) || true
    [ -n "$picked" ] || {
      ui_info "Nothing prepared. The plan is saved."
      return 0
    }
    while IFS= read -r id; do
      [ -z "$id" ] && continue
      chosen+=("$id")
    done <<<"$picked"
  fi

  for id in "${chosen[@]+"${chosen[@]}"}"; do
    local plan modules_line
    plan=$(get_plan "$issue" "$id")
    branch=$(printf '%s' "$plan" | "$JQ" -r '.branch')
    mapfile -t modules_line < <(printf '%s' "$plan" | "$JQ" -r '.modules[]')
    dest=$(prepare_one_plan "$issue" "$base_sha" "$id" "$branch" "${modules_line[@]}")
    plan=$(printf '%s' "$plan" | "$JQ" --arg worktree "$dest" '.worktree = $worktree')
    upsert_plan "$issue" "$plan"
    echo ""
    ui_ok "Prepared ${id}"
    ui_info "  Branch: ${branch}"
    ui_info "  Worktree: ${dest}"
    ui_info "  Next: cd ${dest} && nix run .#commit"
    ui_info "  Then: nix run .#pr -- create"
  done
  echo ""
  ui_info "The original working tree was not modified. Unrelated changes are still there."
  ui_info "This command did not commit, push, or open a pull request."
  rm -f "$edges_file" "$nodes_file"
}

modules_from_range() {
  local base_sha="$1"
  local tip="${2:-HEAD}"
  local file unit
  local -A seen=()
  while IFS= read -r file; do
    [ -z "$file" ] && continue
    unit=$(category_for_path "$file")
    [ -n "$unit" ] || continue
    if [ -z "${seen[$unit]:-}" ]; then
      seen["$unit"]=1
      printf '%s\n' "$unit"
    fi
  done < <("$GIT" diff --name-only "${base_sha}" "${tip}")
}

adopt_branch() {
  local issue="$1"
  local base="$2"
  local base_sha="$3"
  local branch
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    branch=${RIVAAS_ADOPT_BRANCH:-}
  else
    branch=$(
      "$GIT" for-each-ref --format='%(refname:short)' refs/heads/ | grep -vx "$base" | ui_gum choose --header "Branch to adopt" </dev/tty
    ) || true
  fi
  [ -n "$branch" ] || die "No branch selected."
  "$GIT" show-ref --verify --quiet "refs/heads/${branch}" || die "No local branch ${branch}."
  local dest
  dest=$(ensure_worktree "$branch" "$base_sha")
  local -a units=()
  mapfile -t units < <(modules_from_range "$base_sha" "$branch")
  [ "${#units[@]}" -gt 0 ] || die "Branch ${branch} does not change a known module relative to ${base}."
  local id="${units[0]}"
  local modules_json
  modules_json=$(json_array "${units[@]}")
  local plan
  plan=$(make_plan_json "$id" "$modules_json" "$branch" "$dest" '[]' "" true "")
  plan=$(printf '%s' "$plan" | "$JQ" '.adopted = true')
  upsert_issue "$issue" "$ISSUE_TITLE" "$base"
  upsert_plan "$issue" "$plan"
  ui_ok "Adopted ${branch} for ${id}."
  ui_info "No new commit was created. Run 'nix run .#pr -- create' from ${dest} when the branch is ready."
}

link_pr() {
  local issue="$1"
  local base="$2"
  local number
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    number=${RIVAAS_LINK_PR:-}
  else
    number=$(prompt_line "Existing pull request number" "")
  fi
  [[ "$number" =~ ^[0-9]+$ ]] || die "Pull request number must be an integer."
  require_gh
  local json
  json=$("$GH" pr view "$number" --json number,url,state,isDraft,headRefName,title) || die "Could not read pull request #${number}."
  local url state head title
  url=$(printf '%s' "$json" | "$JQ" -r '.url')
  state=$(printf '%s' "$json" | "$JQ" -r '.state')
  head=$(printf '%s' "$json" | "$JQ" -r '.headRefName')
  title=$(printf '%s' "$json" | "$JQ" -r '.title')
  local -a units=()
  local base_sha
  base_sha=$(base_sha_for "$base")
  if "$GIT" show-ref --verify --quiet "refs/heads/${head}"; then
    mapfile -t units < <(modules_from_range "$base_sha" "$head")
  fi
  if [ "${#units[@]}" -eq 0 ]; then
    units=("repo")
  fi
  local plan
  plan=$(make_plan_json "${units[0]}" "$(json_array "${units[@]}")" "$head" "" '[]' "" true "")
  plan=$(printf '%s' "$plan" | "$JQ" --argjson num "$number" --arg url "$url" '.pr_number = $num | .pr_url = $url | .adopted = true')
  upsert_issue "$issue" "$ISSUE_TITLE" "$base"
  upsert_plan "$issue" "$plan"
  ui_ok "Linked pull request #${number} (${state}): ${url}"
  ui_info "Title: ${title}"
  ui_info "No duplicate pull request will be created for branch ${head}."
}

reference_commit() {
  local issue="$1"
  local base="$2"
  local base_sha="$3"
  local sha
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    sha=${RIVAAS_COMMIT:-}
  else
    sha=$(prompt_line "Commit already on ${base}" "")
  fi
  [ -n "$sha" ] || die "Commit SHA is required."
  sha=$("$GIT" rev-parse --verify "${sha}^{commit}") || die "Not a commit: ${sha}"
  if ! "$GIT" merge-base --is-ancestor "$sha" "$base_sha"; then
    die "${sha} is not on ${base}. Open a pull request from its branch instead."
  fi
  local -a units=()
  local parent
  parent=$("$GIT" rev-parse "${sha}^" 2>/dev/null || printf '%s' "$sha")
  mapfile -t units < <(modules_from_range "$parent" "$sha")
  [ "${#units[@]}" -gt 0 ] || units=("repo")
  local id="${units[0]}"
  local plan
  plan=$(make_plan_json "$id" "$(json_array "${units[@]}")" "$base" "" '[]' "" true "$sha")
  upsert_issue "$issue" "$ISSUE_TITLE" "$base"
  upsert_plan "$issue" "$plan"
  ui_ok "Recorded ${sha} on ${base} for ${id}."
  ui_info "No pull request was opened. The commit is already on the default branch."
}

cmd_prepare() {
  local issue="${1:-}"
  [[ "$issue" =~ ^[0-9]+$ ]] || die "Usage: pr prepare <issue-number>"
  require_repo_root || exit 1
  need_gum
  warn_if_linked_worktree
  if repo_has_unmerged; then
    die "The index has unmerged files. Resolve them before preparing a pull request."
  fi
  read_issue "$issue"
  maybe_fetch
  local base base_sha action
  base=$(default_branch_name)
  base_sha=$(base_sha_for "$base") || die "Could not resolve ${base}."
  ui_header "Prepare pull requests for #${issue}"
  ui_info "${ISSUE_TITLE}"
  [ -n "$ISSUE_URL" ] && ui_info "$ISSUE_URL"
  ui_info "Base branch: ${base}"
  echo ""
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    action=${RIVAAS_ACTION:-prepare}
  else
    action=$(ui_gum choose --header "What do you want to do?" \
      "Prepare uncommitted changes" \
      "Adopt an existing branch" \
      "Link an existing pull request" \
      "Reference a commit on ${base}" </dev/tty) || die "No action selected."
  fi
  case "$action" in
    prepare|"Prepare uncommitted changes") prepare_new "$issue" "$base" "$base_sha" ;;
    adopt|"Adopt an existing branch") adopt_branch "$issue" "$base" "$base_sha" ;;
    link|"Link an existing pull request") link_pr "$issue" "$base" ;;
    commit|"Reference a commit on ${base}") reference_commit "$issue" "$base" "$base_sha" ;;
    *) die "Unknown action: ${action}" ;;
  esac
}

render_pr_body() {
  local summary="$1"
  local parent="$2"
  local sub="${3:-}"
  local close_sub="${4:-0}"
  local primary="$5"
  local additional="${6:-None}"
  local breaking="${7:-No}"
  local depends="${8:-None}"
  local testing="${9:-Not recorded yet.}"
  cat <<EOF
# Summary

${summary}

## Related Issue

Refs #${parent}
EOF
  if [ -n "$sub" ] && [ "$close_sub" = 1 ] && [ "$sub" != "$parent" ]; then
    printf '\nCloses #%s\n' "$sub"
  fi
  cat <<EOF

## Module Scope

**Primary module:** ${primary}

**Additional modules:** ${additional}

**Breaking changes:** ${breaking}

## Dependencies

**Depends on:** ${depends}

## Testing

${testing}

## Checklist

- [ ] Relevant tests pass.
- [ ] Changes follow module boundaries and contribution guidelines.
- [ ] Public API or behavior changes are documented.
- [ ] Dependency and breaking-change impacts are identified, if applicable.
EOF
}

current_plan_for_branch() {
  local branch="$1"
  local file
  file=$(state_file)
  [ -f "$file" ] || return 1
  "$JQ" -ce --arg branch "$branch" '
    first(.issues | to_entries[] | .value.plans[]? | select(.branch == $branch))
  ' "$file"
}

issue_for_branch() {
  local branch="$1"
  local file
  file=$(state_file)
  [ -f "$file" ] || return 1
  "$JQ" -er --arg branch "$branch" '
    .issues | to_entries[] | select(any(.value.plans[]?; .branch == $branch)) | .key
  ' "$file" | head -1
}

existing_pr_json() {
  local branch="$1"
  "$GH" pr list --head "$branch" --state all --json number,url,state,isDraft
}

must_draft() {
  local issue="$1"
  local plan="$2"
  local dep
  while IFS= read -r dep; do
    [ -z "$dep" ] && continue
    if ! plan_dependency_ready "$issue" "$dep"; then
      ui_warn "Dependency ${dep} is not merged and released yet. Opening a draft."
      return 0
    fi
  done < <(printf '%s' "$plan" | "$JQ" -r '.depends_on[]?')
  return 1
}

repo_slug() {
  local url
  url=$("$GIT" remote get-url origin 2>/dev/null || true)
  url=${url%.git}
  case "$url" in
    git@github.com:*) url=${url#git@github.com:} ;;
    https://github.com/*) url=${url#https://github.com/} ;;
    ssh://git@github.com/*) url=${url#ssh://git@github.com/} ;;
    *) url="" ;;
  esac
  printf '%s\n' "$url"
}

maybe_subissue() {
  local parent="$1"
  local primary="$2"
  local summary="$3"
  local existing="${4:-}"
  if [ -n "$existing" ] && [ "$existing" != null ]; then
    printf '%s\n' "$existing"
    return 0
  fi
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    [ "${RIVAAS_SUBISSUE:-0}" = 1 ] || return 0
  else
    confirm_default_no "Create an optional sub-issue for ${primary}?" || return 0
  fi
  require_gh
  local url number slug child_id parent_id
  url=$("$GH" issue create --title "[${primary}] ${summary}" --body "Part of #${parent}.") || {
    ui_warn "Could not create a sub-issue. The pull request will only reference #${parent}."
    return 0
  }
  number=${url##*/}
  [[ "$number" =~ ^[0-9]+$ ]] || {
    ui_warn "Could not parse the new issue number from ${url}."
    return 0
  }
  slug=$(repo_slug)
  case "$slug" in
    */*)
      child_id=$("$GH" api "repos/${slug}/issues/${number}" --jq '.id' 2>/dev/null || true)
      parent_id=$("$GH" api "repos/${slug}/issues/${parent}" --jq '.id' 2>/dev/null || true)
      if [ -n "$child_id" ]; then
        if ! "$GH" api --method POST "repos/${slug}/issues/${parent}/sub_issues" -f "sub_issue_id=${child_id}" >/dev/null 2>&1; then
          ui_warn "Issue #${number} was created. Linking it as a sub-issue of #${parent} failed. Link it in the GitHub UI if you want that relationship."
        fi
      fi
      ;;
    *)
      ui_warn "Origin is not a GitHub repository, so #${number} was not linked as a sub-issue."
      ;;
  esac
  printf '%s\n' "$number"
}

default_title() {
  local primary="$1"
  local summary="$2"
  local type slug
  type=${RIVAAS_TYPE:-feat}
  slug=$(slugify "$summary")
  [ -n "$slug" ] || slug=update
  if is_repo_scope "$primary" && [ "$type" = docs ]; then
    printf '%s\n' "docs: ${slug}"
    return 0
  fi
  printf '%s\n' "${type}(${primary}): ${slug}"
}

cmd_create() {
  require_repo_root || exit 1
  need_gum
  local branch base base_sha ahead
  branch=$("$GIT" rev-parse --abbrev-ref HEAD)
  [ "$branch" != HEAD ] || die "Detached HEAD. Check out the feature branch first."
  maybe_fetch
  base=$(default_branch_name)
  if [ "$branch" = "$base" ]; then
    die "You are on ${base}. Open a feature branch first. A commit that already landed on ${base} can be recorded with 'pr prepare'."
  fi
  base_sha=$(base_sha_for "$base") || die "Could not resolve ${base}."
  ahead=$("$GIT" rev-list --count "${base_sha}..HEAD")
  [ "$ahead" -gt 0 ] || die "This branch has no commits beyond ${base}. Commit with 'nix run .#commit' first. This command does not commit."

  require_gh
  local existing_json existing_url existing_state existing_num
  existing_json=$(existing_pr_json "$branch" 2>/dev/null || printf '%s\n' '[]')
  existing_url=$(printf '%s' "$existing_json" | "$JQ" -r '[.[] | select(.state == "OPEN")][0].url // empty')
  if [ -n "$existing_url" ]; then
    ui_ok "Pull request already exists: ${existing_url}"
    exit 0
  fi
  existing_state=$(printf '%s' "$existing_json" | "$JQ" -r '.[0].state // empty')
  if [ "$existing_state" = MERGED ]; then
    die "A merged pull request already exists for ${branch}. No duplicate was created."
  fi
  if [ "$existing_state" = CLOSED ]; then
    die "A closed pull request already exists for ${branch}. Reopen it, or use a new branch. No duplicate was created."
  fi

  local issue plan
  issue=$(issue_for_branch "$branch" 2>/dev/null || true)
  plan=$(current_plan_for_branch "$branch" 2>/dev/null || true)
  if [ -z "$issue" ]; then
    if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
      issue=${RIVAAS_ISSUE:-}
    else
      issue=$(prompt_line "Existing issue number" "")
    fi
  fi
  [[ "$issue" =~ ^[0-9]+$ ]] || die "An existing issue number is required. This command does not create the parent issue."

  local -a units=()
  if [ -n "$plan" ]; then
    mapfile -t units < <(printf '%s' "$plan" | "$JQ" -r '.modules[]')
  else
    mapfile -t units < <(modules_from_range "$base_sha" HEAD)
  fi
  [ "${#units[@]}" -gt 0 ] || die "Could not tell which module this branch changes."
  local primary="${units[0]}"
  local additional="None"
  if [ "${#units[@]}" -gt 1 ]; then
    additional=$(printf '%s' "$(printf '%s\n' "${units[@]:1}")" | paste -sd, -)
  fi
  local justification=""
  if [ -n "$plan" ]; then
    justification=$(printf '%s' "$plan" | "$JQ" -r '.justification // empty')
  fi
  if [ "${#units[@]}" -gt 1 ] && [ -z "$justification" ]; then
    if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
      justification=${RIVAAS_JUSTIFICATION:-}
    else
      justification=$(prompt_line "Why is this more than one module?" "")
    fi
    [ -n "$justification" ] || die "A multi-module pull request needs a one-line reason."
  fi

  local summary
  summary=${ISSUE_TITLE:-}
  if [ -z "$summary" ] && [ -n "$plan" ]; then
    summary=$(issue_title_from_state "$issue")
  fi
  [ -n "$summary" ] || summary=$("$GIT" log -1 --pretty=%s)
  if [ -n "$justification" ]; then
    summary="${summary} ${justification}"
  fi

  local title
  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    title=${RIVAAS_TITLE:-$(default_title "$primary" "$summary")}
  else
    local suggested
    suggested=$(default_title "$primary" "$summary")
    title=$(prompt_line "Pull request title" "$suggested")
  fi
  [ -n "$title" ] || die "A title is required."

  local sub_existing=""
  if [ -n "$plan" ]; then
    sub_existing=$(printf '%s' "$plan" | "$JQ" -r '.sub_issue // empty')
  fi
  local sub_issue close_sub=0
  sub_issue=$(maybe_subissue "$issue" "$primary" "$summary" "$sub_existing" || true)
  if [ -n "$sub_issue" ]; then
    close_sub=1
  fi

  local depends="None"
  if [ -n "$plan" ]; then
    local dep note=""
    while IFS= read -r dep; do
      [ -z "$dep" ] && continue
      if plan_dependency_ready "$issue" "$dep"; then
        note="${note}${dep} (released), "
      else
        note="${note}${dep} (not released yet), "
      fi
    done < <(printf '%s' "$plan" | "$JQ" -r '.depends_on[]?')
    note=${note%, }
    [ -n "$note" ] && depends="$note"
  fi

  local breaking="No"
  local testing="Not recorded yet."
  local body_file
  body_file=$(mktemp)
  render_pr_body "$summary" "$issue" "$sub_issue" "$close_sub" "$primary" "$additional" "$breaking" "$depends" "$testing" >"$body_file"

  if [ "${RIVAAS_NONINTERACTIVE:-}" != 1 ]; then
    ui_info "Edit the pull request body. Save with Ctrl-D."
    local edited
    edited=$(ui_gum write --width 80 --height 24 <"$body_file" </dev/tty) || true
    if [ -n "$edited" ]; then
      printf '%s\n' "$edited" >"$body_file"
    fi
  fi

  local draft=0
  if [ -n "$plan" ] && must_draft "$issue" "$plan"; then
    draft=1
  elif [ "${RIVAAS_DRAFT:-auto}" = yes ]; then
    draft=1
  fi

  if [ "${RIVAAS_NONINTERACTIVE:-}" = 1 ]; then
    [ "${RIVAAS_PUSH:-}" = 1 ] || die "Refusing to push without RIVAAS_PUSH=1. Push the branch yourself, then rerun create."
  else
    confirm_default_yes "Push ${branch} to origin and open the pull request?" || die "Not pushed. Nothing was opened."
  fi
  "$GIT" push -u origin HEAD || die "Push failed. No pull request was created."

  local -a args=(pr create --base "$base" --head "$branch" --title "$title" --body-file "$body_file")
  if [ "$draft" -eq 1 ]; then
    args+=(--draft)
  fi
  local url
  url=$("$GH" "${args[@]}") || die "gh pr create failed."
  local number
  number=$("$GH" pr view --json number --jq '.number' 2>/dev/null || true)
  ui_ok "Pull request: ${url}"
  if [ "$draft" -eq 1 ]; then
    ui_info "Opened as a draft because a dependency is still unpublished or you asked for a draft."
  fi
  ui_info "The parent issue is referenced with Refs #${issue}. This pull request does not close it."

  if [ -n "$plan" ]; then
    plan=$(printf '%s' "$plan" | "$JQ" --arg url "$url" --argjson num "${number:-null}" --arg sub "${sub_issue:-}" '
      .pr_url = $url
      | .pr_number = (if $num == null then .pr_number else $num end)
      | .sub_issue = (if $sub == "" then .sub_issue else ($sub | tonumber) end)
    ')
    upsert_plan "$issue" "$plan"
  fi
  rm -f "$body_file"
}

issue_title_from_state() {
  local issue="$1"
  local file
  file=$(state_file)
  [ -f "$file" ] || return 0
  "$JQ" -r --arg issue "$issue" '.issues[$issue].title // empty' "$file"
}

cmd_status() {
  local issue="${1:-}"
  require_repo_root || exit 1
  local file
  file=$(state_file)
  if [ ! -f "$file" ]; then
    ui_info "No local pull request plans. Run 'nix run .#pr -- prepare <issue>'."
    exit 0
  fi
  ui_header "Pull request status"
  local keys
  if [ -n "$issue" ]; then
    keys=$issue
  else
    keys=$("$JQ" -r '.issues | keys[]' "$file")
  fi
  local key
  while IFS= read -r key; do
    [ -z "$key" ] && continue
    local title base
    title=$("$JQ" -r --arg issue "$key" '.issues[$issue].title // ""' "$file")
    base=$("$JQ" -r --arg issue "$key" '.issues[$issue].base // "main"' "$file")
    echo ""
    ui_info "Issue #${key}: ${title}"
    local open_plans=0
    local total=0
    while IFS= read -r plan; do
      [ -z "$plan" ] && continue
      total=$((total + 1))
      local id branch worktree pr_url pr_num landed deps
      id=$(printf '%s' "$plan" | "$JQ" -r '.id')
      branch=$(printf '%s' "$plan" | "$JQ" -r '.branch')
      worktree=$(printf '%s' "$plan" | "$JQ" -r '.worktree // empty')
      pr_url=$(printf '%s' "$plan" | "$JQ" -r '.pr_url // empty')
      pr_num=$(printf '%s' "$plan" | "$JQ" -r '.pr_number // empty')
      landed=$(printf '%s' "$plan" | "$JQ" -r '.landed_commit // empty')
      deps=$(printf '%s' "$plan" | "$JQ" -r '.depends_on | join(", ")')
      [ -n "$deps" ] || deps=none
      printf '  %s\n' "$id"
      printf '    branch: %s\n' "$branch"
      [ -n "$worktree" ] && printf '    worktree: %s\n' "$worktree"
      printf '    depends on: %s\n' "$deps"
      if [ -n "$landed" ]; then
        printf '    landed commit: %s\n' "$landed"
      elif [ -n "$pr_url" ]; then
        printf '    pull request: %s\n' "$pr_url"
        if [ "${RIVAAS_OFFLINE:-}" != 1 ] && [ -n "$pr_num" ] && { [ -x "$GH" ] || command -v "$GH" >/dev/null 2>&1; }; then
          local live
          live=$("$GH" pr view "$pr_num" --json state,isDraft --jq 'if .isDraft then "DRAFT" else .state end' 2>/dev/null || true)
          [ -n "$live" ] && printf '    state: %s\n' "$live"
          if [ "$live" != MERGED ]; then
            open_plans=$((open_plans + 1))
          fi
        else
          open_plans=$((open_plans + 1))
        fi
      else
        printf '    pull request: not opened\n'
        open_plans=$((open_plans + 1))
      fi
      local dep ready=1
      while IFS= read -r dep; do
        [ -z "$dep" ] && continue
        if plan_dependency_ready "$key" "$dep"; then
          printf '    dependency %s: published\n' "$dep"
        else
          printf '    dependency %s: not published yet\n' "$dep"
          ready=0
        fi
      done < <(printf '%s' "$plan" | "$JQ" -r '.depends_on[]?')
      if [ "$ready" -eq 0 ]; then
        printf '    not merge-ready until those dependencies are released\n'
      fi
    done < <("$JQ" -c --arg issue "$key" '.issues[$issue].plans[]?' "$file")
    echo ""
    if [ "$total" -eq 0 ]; then
      ui_info "No plans recorded."
    elif [ "$open_plans" -eq 0 ]; then
      ui_info "Every recorded plan for #${key} is merged or already on ${base}."
      ui_info "Close #${key} only after you have checked that the whole feature is done. This command will not close it."
    else
      ui_info "Issue #${key} stays open. ${open_plans} plan(s) are not merged yet."
      ui_info "Do not put Fixes #${key} on an intermediate pull request."
    fi
  done <<<"$keys"
}

cmd_help() {
  cat <<'EOF'
Usage: nix run .#pr -- <command>

  prepare <issue-number>   Select module changes for an existing issue
  create                    Open or reuse a pull request for the current branch
  status [issue-number]    Show recorded plans, dependencies, and pull requests

prepare copies the selected module into a new worktree and leaves the
original working tree alone. It does not commit, push, or release.
Commit with: nix run .#commit
Release with: nix run .#release

Contributors without Nix can follow CONTRIBUTING.md. The pull request
template and the PR Policy check are the same either way.
EOF
}

main() {
  local cmd="${1:-help}"
  if [ "$#" -gt 0 ]; then
    shift
  fi
  case "$cmd" in
    prepare) cmd_prepare "$@" ;;
    create) cmd_create "$@" ;;
    status) cmd_status "$@" ;;
    help|-h|--help) cmd_help ;;
    *) die "Unknown command: ${cmd}. Run with help." ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
