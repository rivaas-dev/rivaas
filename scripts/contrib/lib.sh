#!/usr/bin/env bash
# Shared git and module helpers for the commit tool and the PR tool.
# Source this file from bash. Do not execute it directly.
if [ -z "${BASH_VERSION:-}" ]; then
  printf 'scripts/contrib/lib.sh must be sourced from bash\n' >&2
  return 1 2>/dev/null || exit 1
fi
#
# Callers may set GIT, FIND, GUM, and JQ to absolute tool paths.
# Functions assume the current directory is the repository root unless noted.

GIT="${GIT:-git}"
FIND="${FIND:-find}"
JQ="${JQ:-jq}"

# Same xterm colors as nix/lib/colors.nix.
C_HEADER=183
C_SUCCESS=158
C_ERROR=210
C_WARN=222
C_INFO=250
C_ACCENT1=153
C_ACCENT4=229

ui_gum() {
  if [ -n "${GUM:-}" ]; then
    "$GUM" "$@"
  elif [ -n "${gum:-}" ]; then
    "$gum" "$@"
  elif command -v gum >/dev/null 2>&1; then
    command gum "$@"
  else
    return 1
  fi
}

ui_error() {
  if ui_gum style --foreground "$C_ERROR" "$*" >&2; then
    return 0
  fi
  printf 'error: %s\n' "$*" >&2
}

ui_info() {
  if ui_gum style --foreground "$C_INFO" "$*" >&2; then
    return 0
  fi
  printf '%s\n' "$*" >&2
}

ui_warn() {
  if ui_gum style --foreground "$C_WARN" "$*" >&2; then
    return 0
  fi
  printf 'warning: %s\n' "$*" >&2
}

ui_ok() {
  if ui_gum style --foreground "$C_SUCCESS" "$*" >&2; then
    return 0
  fi
  printf '%s\n' "$*" >&2
}

ui_header() {
  if ui_gum style --foreground "$C_HEADER" --bold --border rounded --padding "0 1" "$*" >&2; then
    return 0
  fi
  printf '== %s ==\n' "$*" >&2
}

require_repo_root() {
  local root
  if ! root=$("$GIT" rev-parse --show-toplevel 2>/dev/null); then
    ui_error "Not in a git repository"
    return 1
  fi
  cd "$root" || return 1
}

# True for the single non-module group. Nix, CI, docs, and other
# repository-wide files share this group.
is_contrib() {
  [ "$1" = contrib ]
}

# Example, benchmark, and integration trees support a parent module.
# The path test matches nix/lib/find-patterns.nix rootModules.
is_auxiliary_dir() {
  case "/$1/" in
    */examples/*|*/example/*|*/benchmarks/*|*/integration/*) return 0 ;;
    *) return 1 ;;
  esac
}

# Publishable module directories. A go.mod in an auxiliary tree does not
# start a new module. A deleted go.mod still counts until that deletion is committed.
list_publishable_modules() {
  local gomod dir
  "$FIND" . -name go.mod -type f \
    ! -path '*/examples/*' \
    ! -path '*/example/*' \
    ! -path '*/benchmarks/*' \
    ! -path '*/integration/*' \
    -exec dirname {} \; | sed 's|^\./||'
  while IFS= read -r -d '' gomod; do
    case "$gomod" in
      go.mod|*/go.mod) ;;
      *) continue ;;
    esac
    dir=$(dirname -- "$gomod")
    [ "$dir" = "." ] && continue
    is_auxiliary_dir "$dir" && continue
    printf '%s\n' "$dir"
  done < <("$GIT" ls-files -d -z)
}

find_modules() {
  list_publishable_modules | LC_ALL=C sort -u
  printf '%s\n' contrib
}

# True when this directory is a publishable module in the worktree or in HEAD.
publishable_module_dir() {
  local dir="$1"
  [ -n "$dir" ] && [ "$dir" != "." ] || return 1
  is_auxiliary_dir "$dir" && return 1
  if [ -f "$dir/go.mod" ]; then
    return 0
  fi
  "$GIT" cat-file -e "HEAD:$dir/go.mod" 2>/dev/null
}

# Nearest publishable module. Auxiliary go.mod directories are skipped so
# app/examples and router/benchmarks stay with their parent.
owning_module() {
  local path="$1"
  local dir
  dir=$(dirname -- "$path")
  [ "$dir" = "." ] && return 0
  while [ -n "$dir" ] && [ "$dir" != "." ] && [ "$dir" != "/" ]; do
    if publishable_module_dir "$dir"; then
      printf '%s\n' "$dir"
      return 0
    fi
    dir=$(dirname -- "$dir")
  done
}

# Publishable module, or contrib when no parent module owns the path.
category_for_path() {
  local path="$1"
  local owner
  [ -n "$path" ] || return 0
  owner=$(owning_module "$path")
  if [ -n "$owner" ]; then
    printf '%s\n' "$owner"
    return 0
  fi
  printf '%s\n' contrib
}

# Map a stored group name onto the current publishable module.
# Old plans may name an example or benchmark directory.
canonical_group() {
  local name="$1"
  local parent
  [ -n "$name" ] || return 0
  if is_contrib "$name"; then
    printf '%s\n' contrib
    return 0
  fi
  if is_auxiliary_dir "$name"; then
    parent=$(category_for_path "$name/go.mod")
    if [ -n "$parent" ]; then
      printf '%s\n' "$parent"
      return 0
    fi
    printf '%s\n' contrib
    return 0
  fi
  printf '%s\n' "$name"
}

# JSON object of auxiliary directory -> parent group. Used to rewrite old plans.
auxiliary_parent_json() {
  local gomod dir parent
  local tmp
  tmp=$(mktemp)
  while IFS= read -r gomod; do
    [ -n "$gomod" ] || continue
    dir=$(dirname -- "$gomod")
    dir=${dir#./}
    is_auxiliary_dir "$dir" || continue
    parent=$(category_for_path "$dir/go.mod")
    [ -n "$parent" ] || parent=contrib
    printf '%s\t%s\n' "$dir" "$parent"
  done < <("$FIND" . -name go.mod -type f) >"$tmp"
  if [ ! -s "$tmp" ]; then
    rm -f -- "$tmp"
    printf '%s\n' '{}'
    return 0
  fi
  "$JQ" -R -s '
    split("\n")
    | map(select(length > 0) | split("\t") | {(.[0]): .[1]})
    | add // {}
  ' "$tmp"
  rm -f -- "$tmp"
}

# Fill STAT_* from git status. Includes individual untracked files (-uall)
# so a new directory is not collapsed into one entry.
load_status() {
  STAT_XY=()
  STAT_PATH=()
  STAT_OLD=()
  local -a recs=()
  local i=0 xy path old
  mapfile -d '' recs < <("$GIT" status --porcelain=v1 -z -uall)
  while [ "$i" -lt "${#recs[@]}" ]; do
    xy=${recs[$i]}
    i=$((i + 1))
    [ -z "$xy" ] && continue
    path=${xy:3}
    xy=${xy:0:2}
    old=""
    case "$xy" in
      R?|C?|?R|?C)
        old=$path
        path=${recs[$i]}
        i=$((i + 1))
        ;;
    esac
    STAT_XY+=("$xy")
    STAT_PATH+=("$path")
    STAT_OLD+=("$old")
  done
}

status_count() {
  printf '%s\n' "${#STAT_PATH[@]}"
}

# kind for one status row: staged, unstaged, both, deleted, untracked, renamed, unmerged, skip
status_kind() {
  local xy="$1"
  case "$xy" in
    UU|AA|DD|AU|UA|DU|UD) printf '%s\n' unmerged ;;
    '??') printf '%s\n' untracked ;;
    AD) printf '%s\n' skip ;;
    R?|C?|?R|?C) printf '%s\n' renamed ;;
    D?) printf '%s\n' deleted ;;
    ?D) printf '%s\n' deleted ;;
    '  ') printf '%s\n' skip ;;
    ?' ') printf '%s\n' staged ;;
    ' '?) printf '%s\n' unstaged ;;
    *) printf '%s\n' both ;;
  esac
}

path_in_unit() {
  local unit="$1"
  local path="$2"
  local got
  got=$(category_for_path "$path")
  [ "$got" = "$unit" ]
}

# NUL triples: kind, path, extra.
# extra is the old path for renames and empty otherwise.
list_module_changes() {
  local unit="$1"
  local i xy path old kind side
  load_status
  for i in "${!STAT_PATH[@]}"; do
    xy=${STAT_XY[$i]}
    path=${STAT_PATH[$i]}
    old=${STAT_OLD[$i]}
    kind=$(status_kind "$xy")
    [ "$kind" = skip ] && continue
    if [ "$kind" = deleted ]; then
      case "$xy" in
        ' '?) kind=unstaged ;;
        *) kind=staged ;;
      esac
    fi
    side=0
    if path_in_unit "$unit" "$path"; then
      side=1
    fi
    if [ -n "$old" ] && path_in_unit "$unit" "$old"; then
      side=1
    fi
    [ "$side" -eq 1 ] || continue
    printf '%s\0%s\0%s\0' "$kind" "$path" "$old"
  done
}

# NUL-delimited paths that a module commit should stage.
list_module_paths() {
  local unit="$1"
  local kind path old
  while IFS= read -r -d '' kind && IFS= read -r -d '' path && IFS= read -r -d '' old; do
    case "$kind" in
      renamed)
        [ -n "$old" ] && printf '%s\0' "$old"
        printf '%s\0' "$path"
        ;;
      unmerged|skip) ;;
      *) printf '%s\0' "$path" ;;
    esac
  done < <(list_module_changes "$unit")
}

count_changes() {
  local unit="$1"
  local staged=0 unstaged=0 untracked=0
  local kind path old
  while IFS= read -r -d '' kind && IFS= read -r -d '' path && IFS= read -r -d '' old; do
    case "$kind" in
      staged|deleted|renamed) staged=$((staged + 1)) ;;
      unstaged) unstaged=$((unstaged + 1)) ;;
      untracked) untracked=$((untracked + 1)) ;;
      both) staged=$((staged + 1)); unstaged=$((unstaged + 1)) ;;
      unmerged) ;;
    esac
  done < <(list_module_changes "$unit")
  printf '%s %s %s\n' "$staged" "$unstaged" "$untracked"
}

print_module_changes() {
  local unit="$1"
  local kind path old label
  while IFS= read -r -d '' kind && IFS= read -r -d '' path && IFS= read -r -d '' old; do
    case "$kind" in
      staged)
        if [ ! -e "$path" ] && [ ! -L "$path" ]; then
          label="[deleted] $path"
        else
          label="[staged] $path"
        fi
        ;;
      unstaged)
        if [ ! -e "$path" ] && [ ! -L "$path" ]; then
          label="[deleted] $path"
        else
          label="[modified] $path"
        fi
        ;;
      both) label="[staged+modified] $path" ;;
      deleted) label="[deleted] $path" ;;
      untracked) label="[untracked] $path" ;;
      renamed) label="[renamed] $old -> $path" ;;
      unmerged) label="[unmerged] $path" ;;
      *) label="[$kind] $path" ;;
    esac
    case "$kind" in
      staged|renamed) ui_gum style --foreground "$C_SUCCESS" "  $label" || printf '  %s\n' "$label" ;;
      unstaged|both) ui_gum style --foreground "$C_ACCENT4" "  $label" || printf '  %s\n' "$label" ;;
      deleted|unmerged) ui_gum style --foreground "$C_ERROR" "  $label" || printf '  %s\n' "$label" ;;
      *) ui_gum style --foreground "$C_ACCENT1" "  $label" || printf '  %s\n' "$label" ;;
    esac
  done < <(list_module_changes "$unit")
}

# Prints a warning and returns 0 when files match no module and no category.
check_outside_changes() {
  local _modules="${1:-}"
  local outside=0
  local i path old owner
  load_status
  for i in "${!STAT_PATH[@]}"; do
    path=${STAT_PATH[$i]}
    old=${STAT_OLD[$i]}
    owner=$(category_for_path "$path")
    if [ -z "$owner" ] && [ -n "$old" ]; then
      owner=$(category_for_path "$old")
    fi
    if [ -z "$owner" ]; then
      outside=$((outside + 1))
    fi
  done
  if [ "$outside" -gt 0 ]; then
    echo ""
    ui_warn "  $outside file(s) changed outside known modules"
    return 0
  fi
  return 1
}

repo_has_unmerged() {
  local i kind
  load_status
  for i in "${!STAT_PATH[@]}"; do
    kind=$(status_kind "${STAT_XY[$i]}")
    if [ "$kind" = unmerged ]; then
      return 0
    fi
  done
  return 1
}

# Renames and copies whose endpoints belong to different units.
# Prints "old<TAB>new<TAB>old_unit<TAB>new_unit" for each one.
# Uses a temporary index so untracked files participate in rename detection
# without changing the real index.
collect_cross_renames() {
  local tmp
  local -a recs=()
  local i=0 st old new ou nu
  tmp=$(mktemp)
  rm -f "$tmp"
  GIT_INDEX_FILE=$tmp "$GIT" read-tree HEAD
  GIT_INDEX_FILE=$tmp "$GIT" add -A
  mapfile -d '' recs < <(GIT_INDEX_FILE=$tmp "$GIT" diff -M --cached --name-status -z HEAD)
  rm -f "$tmp"
  while [ "$i" -lt "${#recs[@]}" ]; do
    st=${recs[$i]}
    i=$((i + 1))
    case "$st" in
      R*|C*)
        old=${recs[$i]}; i=$((i + 1))
        new=${recs[$i]}; i=$((i + 1))
        ou=$(category_for_path "$old")
        nu=$(category_for_path "$new")
        if [ -n "$ou" ] && [ -n "$nu" ] && [ "$ou" != "$nu" ]; then
          printf '%s\t%s\t%s\t%s\n' "$old" "$new" "$ou" "$nu"
        fi
        ;;
      *)
        i=$((i + 1))
        ;;
    esac
  done
}

# Returns 1 when a rename crosses the boundary of the selected units.
# Arguments are unit names. With no arguments, any cross-unit rename fails.
selection_is_safe() {
  local -a selected=("$@")
  local line old new ou nu
  local hit_old hit_new
  local unsafe=0
  local s
  if repo_has_unmerged; then
    ui_error "The index has unmerged files. Resolve them, then try again."
    return 1
  fi
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    old=${line%%$'\t'*}
    new=${line#*$'\t'}
    ou=${new#*$'\t'}
    new=${new%%$'\t'*}
    nu=${ou#*$'\t'}
    ou=${ou%%$'\t'*}
    if [ "${#selected[@]}" -eq 0 ]; then
      unsafe=1
      printf 'rename crosses modules: %s (%s) -> %s (%s)\n' "$old" "$ou" "$new" "$nu" >&2
      continue
    fi
    hit_old=0
    hit_new=0
    for s in "${selected[@]}"; do
      [ "$s" = "$ou" ] && hit_old=1
      [ "$s" = "$nu" ] && hit_new=1
    done
    if [ "$hit_old" -ne "$hit_new" ]; then
      unsafe=1
      printf 'rename crosses modules: %s (%s) -> %s (%s)\n' "$old" "$ou" "$new" "$nu" >&2
    fi
  done < <(collect_cross_renames)
  [ "$unsafe" -eq 0 ]
}

# Stage and commit only the given paths.
# Previously staged paths that are not listed stay staged, including partial hunks.
commit_paths() {
  local msgfile="$1"
  shift
  if [ "$#" -eq 0 ]; then
    ui_error "No files to commit"
    return 1
  fi
  if [ ! -f "$msgfile" ]; then
    ui_error "Commit message file is missing"
    return 1
  fi
  "$GIT" add -- "$@"
  "$GIT" commit -F "$msgfile" --only -- "$@"
}

module_diff_text() {
  local unit="$1"
  local limit="${2:-20000}"
  local tmp kind path old
  local -a tracked=() untracked=()
  tmp=$(mktemp)
  while IFS= read -r -d '' kind && IFS= read -r -d '' path && IFS= read -r -d '' old; do
    case "$kind" in
      untracked) untracked+=("$path") ;;
      renamed)
        [ -n "$old" ] && tracked+=("$old")
        tracked+=("$path")
        ;;
      unmerged|skip) ;;
      *) tracked+=("$path") ;;
    esac
  done < <(list_module_changes "$unit")
  {
    if [ "${#tracked[@]}" -gt 0 ]; then
      "$GIT" diff HEAD -- "${tracked[@]}" || true
    fi
    local u
    for u in "${untracked[@]+"${untracked[@]}"}"; do
      "$GIT" diff --no-index -- /dev/null "$u" || true
    done
  } >"$tmp" || true
  RIVAAS_DIFF_FULL_SIZE=$(wc -c <"$tmp" | tr -d ' ')
  if [ "$RIVAAS_DIFF_FULL_SIZE" -gt "$limit" ]; then
    RIVAAS_DIFF_TRUNCATED=1
  else
    RIVAAS_DIFF_TRUNCATED=0
  fi
  head -c "$limit" "$tmp"
  rm -f "$tmp"
}

# Copy listed worktree paths onto a clean tree. Missing source paths are deletions.
# Binary-safe. Does not modify the source tree.
copy_paths_to_tree() {
  local dest="$1"
  shift
  local p src dst
  [ -d "$dest" ] || {
    ui_error "Destination tree does not exist: $dest"
    return 1
  }
  for p in "$@"; do
    src="$PWD/$p"
    dst="$dest/$p"
    if [ -e "$src" ] || [ -L "$src" ]; then
      rm -rf -- "$dst"
      mkdir -p -- "$(dirname -- "$dst")"
      cp -a -- "$src" "$dst"
    else
      rm -rf -- "$dst"
    fi
  done
}

isolate_units() {
  local dest="$1"
  shift
  local -a units=("$@")
  local -a paths=()
  local unit p
  if [ "${#units[@]}" -eq 0 ]; then
    ui_error "No modules selected to isolate"
    return 1
  fi
  if ! selection_is_safe "${units[@]}"; then
    ui_error "Refusing to isolate changes that cross module boundaries."
    return 1
  fi
  for unit in "${units[@]}"; do
    while IFS= read -r -d '' p; do
      paths+=("$p")
    done < <(list_module_paths "$unit")
  done
  if [ "${#paths[@]}" -eq 0 ]; then
    ui_error "Selected modules have no file changes"
    return 1
  fi
  copy_paths_to_tree "$dest" "${paths[@]}"
}

worktree_base_dir() {
  local common id
  common=$("$GIT" rev-parse --git-common-dir)
  case "$common" in
    /*) ;;
    *) common="$PWD/$common" ;;
  esac
  id=$(printf '%s' "$common" | sha256sum | awk '{print $1}' | cut -c1-12)
  if [ -n "${RIVAAS_WORKTREE_ROOT:-}" ]; then
    printf '%s\n' "$RIVAAS_WORKTREE_ROOT/$id"
  else
    printf '%s\n' "${XDG_STATE_HOME:-$HOME/.local/state}/rivaas/worktrees/$id"
  fi
}

worktree_path_for_branch() {
  local branch="$1"
  local safe base
  safe=$(printf '%s' "$branch" | tr '/ ' '--')
  base=$(worktree_base_dir)
  printf '%s/%s\n' "$base" "$safe"
}

state_file() {
  local common
  common=$("$GIT" rev-parse --git-common-dir)
  case "$common" in
    /*) ;;
    *) common="$PWD/$common" ;;
  esac
  printf '%s/rivaas/pr-state.json\n' "$common"
}

ensure_state_file() {
  local f
  f=$(state_file)
  mkdir -p -- "$(dirname -- "$f")"
  if [ ! -f "$f" ]; then
    printf '%s\n' '{"issues":{}}' >"$f"
  fi
  printf '%s\n' "$f"
}

# ---- module dependency graph ----

declare -gA RIVAAS_MODPATH=()

load_module_index() {
  local dir mpath
  RIVAAS_MODPATH=()
  while IFS= read -r dir; do
    [ -z "$dir" ] && continue
    [ -f "$dir/go.mod" ] || continue
    mpath=$(sed -n 's/^module[[:space:]]\+//p' "$dir/go.mod" | head -1)
    [ -n "$mpath" ] || continue
    RIVAAS_MODPATH["$mpath"]=$dir
  done < <(list_publishable_modules)
}

# Print module directories that this unit requires via go.mod.
declared_dep_dirs() {
  local unit="$1"
  local gomod="$unit/go.mod"
  local mpath dir
  [ -f "$gomod" ] || return 0
  load_module_index
  while IFS= read -r mpath; do
    [ -n "$mpath" ] || continue
    dir=${RIVAAS_MODPATH[$mpath]:-}
    [ -n "$dir" ] || continue
    [ "$dir" = "$unit" ] && continue
    printf '%s\n' "$dir"
  done < <(awk '
    /^require[[:space:]]*\(/ { block=1; next }
    /^\)/ { block=0; next }
    block && $1 ~ /^rivaas\.dev\// { print $1; next }
    /^require[[:space:]]+rivaas\.dev\// { print $2 }
  ' "$gomod")
}

# Kahn topological sort.
# Arguments: nodes (one per line file) and edges "dependent<TAB>dependency".
# Prints nodes with dependencies first. Returns 2 on a cycle.
topo_sort() {
  local nodes_file="$1"
  local edges_file="$2"
  local -a nodes=()
  local -A indeg=()
  local -A deps_of=()
  local node dep line ready progress next
  local -a queue=()
  local -a out=()
  while IFS= read -r node; do
    [ -z "$node" ] && continue
    nodes+=("$node")
    indeg["$node"]=0
  done <"$nodes_file"
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    node=${line%%$'\t'*}
    dep=${line#*$'\t'}
    indeg["$node"]=$((${indeg[$node]:-0} + 1))
    deps_of["$dep"]+="$node"$'\n'
  done <"$edges_file"
  for node in "${nodes[@]}"; do
    if [ "${indeg[$node]}" -eq 0 ]; then
      queue+=("$node")
    fi
  done
  while [ "${#queue[@]}" -gt 0 ]; do
    node=${queue[0]}
    queue=("${queue[@]:1}")
    out+=("$node")
    while IFS= read -r next; do
      [ -z "$next" ] && continue
      indeg["$next"]=$((${indeg[$next]} - 1))
      if [ "${indeg[$next]}" -eq 0 ]; then
        queue+=("$next")
      fi
    done <<<"${deps_of[$node]:-}"
  done
  if [ "${#out[@]}" -ne "${#nodes[@]}" ]; then
    printf '%s\n' "dependency cycle" >&2
    return 2
  fi
  printf '%s\n' "${out[@]}"
}

# Nodes with no unmet dependencies, preserving the input node order.
ready_nodes() {
  local nodes_file="$1"
  local edges_file="$2"
  local -A blocked=()
  local line node
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    node=${line%%$'\t'*}
    blocked["$node"]=1
  done <"$edges_file"
  while IFS= read -r node; do
    [ -z "$node" ] && continue
    [ -n "${blocked[$node]:-}" ] && continue
    printf '%s\n' "$node"
  done <"$nodes_file"
}

slugify() {
  local s="$1"
  s=$(printf '%s' "$s" | tr '[:upper:]' '[:lower:]')
  s=$(printf '%s' "$s" | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g')
  printf '%s\n' "$s" | cut -c1-40
}

default_branch_name() {
  local b
  b=$("$GIT" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)
  b=${b#origin/}
  if [ -n "$b" ]; then
    printf '%s\n' "$b"
    return 0
  fi
  if "$GIT" show-ref --verify --quiet refs/heads/main; then
    printf '%s\n' main
    return 0
  fi
  if "$GIT" show-ref --verify --quiet refs/heads/master; then
    printf '%s\n' master
    return 0
  fi
  printf '%s\n' main
}

# Resolve the published base for a local branch.
# Prefers the branch's configured upstream (main@{upstream}) over origin,
# because origin is not always the remote that publishes this repository.
# Sets BASE_BRANCH, BASE_REMOTE, BASE_REF, BASE_LOCAL_SHA, BASE_REMOTE_SHA.
load_base_context() {
  local branch="${1:-}"
  local upstream=""
  if [ -z "$branch" ]; then
    branch=$(default_branch_name)
  fi
  BASE_BRANCH=$branch
  BASE_REMOTE=""
  BASE_REF=""
  BASE_LOCAL_SHA=""
  BASE_REMOTE_SHA=""
  if "$GIT" show-ref --verify --quiet "refs/heads/${branch}"; then
    BASE_LOCAL_SHA=$("$GIT" rev-parse "refs/heads/${branch}")
    upstream=$("$GIT" rev-parse --abbrev-ref "${branch}@{upstream}" 2>/dev/null || true)
  else
    BASE_LOCAL_SHA=$("$GIT" rev-parse HEAD)
  fi
  if [ -n "$upstream" ] && [[ "$upstream" == */* ]] && [[ "$upstream" != *"@{upstream}"* ]]; then
    BASE_REMOTE=${upstream%%/*}
    BASE_REF="refs/remotes/${upstream}"
  elif "$GIT" remote get-url origin >/dev/null 2>&1; then
    BASE_REMOTE=origin
    BASE_REF="refs/remotes/origin/${branch}"
  fi
  if [ -n "$BASE_REF" ] && "$GIT" show-ref --verify --quiet "$BASE_REF"; then
    BASE_REMOTE_SHA=$("$GIT" rev-parse "$BASE_REF")
  fi
}

base_sha_for() {
  local branch="$1"
  load_base_context "$branch"
  if [ -n "$BASE_REMOTE_SHA" ]; then
    printf '%s\n' "$BASE_REMOTE_SHA"
    return 0
  fi
  if [ -n "$BASE_LOCAL_SHA" ]; then
    printf '%s\n' "$BASE_LOCAL_SHA"
    return 0
  fi
  return 1
}

# True when a tag for the module contains the given commit.
tag_contains_commit() {
  local unit="$1"
  local sha="$2"
  local tag
  [ -n "$sha" ] || return 1
  tag=$("$GIT" tag -l "$unit/v*" --sort=-v:refname | head -1)
  [ -n "$tag" ] || return 1
  "$GIT" merge-base --is-ancestor "$sha" "$tag^{commit}" 2>/dev/null
}

clean_ai_output() {
  printf '%s\n' "$1" | sed 's/^"//;s/"$//' | sed "s/^'//;s/'$//" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

strip_markdown() {
  sed '/^```/d'
}
