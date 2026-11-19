# Contributing to Rivaas

Rivaas is a multi-module Go repository. Each module has its own `go.mod`, its own version, and its own Git tag. A change in one module should land in its own commit and, by default, its own pull request.

This guide covers the whole path: local work, commits, pull requests, review, and releases. Nix helpers are optional. A pull request opened by hand uses the same template and the same checks.

## Contents

- [Repository layout](#repository-layout)
- [What you need](#what-you-need)
- [Nix workflow](#nix-workflow)
- [Manual workflow](#manual-workflow)
- [Commits](#commits)
- [Pull requests](#pull-requests)
- [Tests and quality checks](#tests-and-quality-checks)
- [Dependencies across modules](#dependencies-across-modules)
- [Releases](#releases)
- [Troubleshooting](#troubleshooting)

## Repository layout

The modules that are published live in their own directories:

| Directory | Module path |
| --- | --- |
| `app/` | `rivaas.dev/app` |
| `router/` | `rivaas.dev/router` |
| `binding/` | `rivaas.dev/binding` |
| `validation/` | `rivaas.dev/validation` |
| `config/` | `rivaas.dev/config` |
| `logging/` | `rivaas.dev/logging` |
| `metrics/` | `rivaas.dev/metrics` |
| `tracing/` | `rivaas.dev/tracing` |
| `openapi/` | `rivaas.dev/openapi` |
| `errors/` | `rivaas.dev/errors` |
| `middleware/<name>/` | `rivaas.dev/middleware/<name>` |

`go.work` ties those modules together on your machine. It is for local development. People who `go get` a released module do not use this workspace.

A directory with its own `go.mod` is its own module, even when it sits inside another directory. `router/benchmarks` is not part of `router`. `middleware/cors` is not part of a single middleware module. `app/examples/02-blog` is an example module, not part of `app`.

Module boundaries matter because each module is versioned on its own. A commit that mixes `router` and `app` is hard to tag, hard to review, and easy to release in the wrong order.

Two kinds of dependency show up in this repo:

- A **declared dependency** is a `require` line in `go.mod`. `app` requires a released `router` version.
- A **change-level dependency** means the code you are writing needs behavior that is not in a released tag yet.

A declared dependency does not mean your current edit has to wait. A change-level dependency does.

## What you need

Install Go 1.27 or newer for the full workspace. `go.work` and `app/go.mod` require Go 1.27. Most other modules declare Go 1.25, so a change that stays inside one of those modules can be tested with Go 1.25. Use Go 1.27 if you build `app` or use `go.work`.

You also need Git. To open a pull request from the terminal, install the [GitHub CLI](https://cli.github.com/) (`gh`) and run `gh auth login`. The GitHub website works without `gh`.

Nix is the recommended way to get the same tools CI uses. From the repository root:

```bash
nix develop
```

The development shell provides Go 1.27, `golangci-lint`, `git`, `gh`, `gum`, and `jq`. Cursor is optional. The commit helper can ask Cursor Agent for a message. You can type the message yourself.

These commands are the helpers. They are not required:

```bash
nix run .#commit-check
nix run .#commit
nix run .#pr -- prepare 123
nix run .#pr -- create
nix run .#pr -- status
nix run .#release-check
nix run .#release
```

Pass arguments after `--`. Nix otherwise treats them as its own flags.

## Nix workflow

Start from an existing GitHub issue. The helper does not create that issue.

```bash
nix run .#pr -- prepare 123
```

The command reads the issue, looks at your branch and working tree, and lists modules with uncommitted changes. Pick the modules that belong to this issue. The default is one pull request per module. If you put several modules in one pull request, the helper asks for a one-line reason.

It then asks which of those changes need new code that is not released yet. Answer from the work you actually did. A `require` line in `go.mod` is only a hint.

For each module that can move now, prepare copies that module's files into a new Git worktree and a new branch. Your original checkout stays as it is, including the copied edits and any staged files from other modules. After the new branch has its commit, remove those copied paths from the original checkout yourself. The helper does not reset your tree. Nothing is committed, pushed, or tagged.

Example: the tree has edits in `binding`, `router`, `app`, and `logging`, and you select `router`. The router worktree receives the router files. The other edits stay in the original tree.

If `app` needs the new router behavior, `app` is recorded and left where it is until that router change is merged and tagged.

Next, in the worktree the command printed:

```bash
cd /path/to/the/worktree
nix run .#commit
```

`nix run .#commit` is the commit tool. It stages the selected module and commits with `git commit --only`, so staged files from another module stay staged and stay out of this commit. Read the message before you accept it. A generated message is a draft.

Then:

```bash
nix run .#pr -- create
```

Create compares the branch with the default branch, refuses to open a second pull request for the same branch, and fills in the pull request template. You can edit the title and the body. The title looks like `feat(router): add configurable timeout`.

The body uses `Refs #123` for the parent issue. It does not use `Fixes` or `Closes` on the parent. An intermediate pull request must leave the parent issue open.

If you want a sub-issue for this module, say yes when asked. Sub-issues are optional. A pull request that finishes its own sub-issue may say `Closes #124`. It still says `Refs #123` for the parent.

If this branch depends on a module that is not merged and tagged yet, create opens a **draft**. A draft is not a request to merge. After the dependency is released, update the consumer's `go.mod`, push, and mark the pull request ready.

Check progress with:

```bash
nix run .#pr -- status 123
```

Status reads Git and GitHub. It does not close the parent issue. Close the parent yourself when every planned pull request has merged and any required module tag exists.

If the work already exists, prepare can adopt it:

- a local branch with commits
- a pull request that is already open
- a commit that is already on the default branch

Adopting records that work. It does not make a replacement commit or a second pull request.

## Manual workflow

You can do the same work with Git, Go, and GitHub. Fork the repository if you do not have write access, then clone your fork.

### Branch

```bash
git fetch origin
git checkout -b feat/router/123-configurable-timeout origin/main
```

Branch names are free-form. `type/module/issue-short-description` is easy to recognize later. Keep the branch limited to one module unless you have a reason to combine modules.

### Change and test

Edit files inside that module. From the module directory:

```bash
go test -C router ./...
```

`go test ./...` at the repository root does not work. There is no root module, and `go.work` does not make `./...` mean "every module".

### Commit

Stage only the module you are committing:

```bash
git add router
git status
git commit --only -- router
```

`git commit` with no path commits every staged file. If `app` was already staged, that commit would include it. `git commit --only -- router` commits the router paths and leaves other staged files staged.

A nested module stays out of the parent. Do not `git add router` if you also changed `router/benchmarks`. Add the files that belong to `router` itself.

Commit messages in this repository look like this:

```text
router: add configurable timeout
```

For a large change, add a blank line and a few bullet points that start with a lowercase verb:

```text
router: add configurable timeout

- read the timeout from route options
- return 504 when the handler exceeds it
```

The first line is short, lowercase, and imperative. The prefix is the module directory (`router`, `middleware/cors`) or a repository area (`nix`, `ci`, `docs`, `deps`). A commit is not a release.

If you already committed on `main` or on another branch, do not redo the commit. Open the pull request from that branch, or mention the commit on the issue if it is already on the default branch.

### Push and open the pull request

```bash
git push -u origin HEAD
gh pr create --base main --title "feat(router): add configurable timeout" --body-file /tmp/pr-body.md
```

You can also push and open the pull request on github.com. GitHub loads [`.github/pull_request_template.md`](.github/pull_request_template.md). Fill it in. Delete the HTML comments if you want a shorter page. The policy check ignores those comments.

Use `Refs #123` in **Related Issue**. Use `Closes #124` only when this pull request finishes that sub-issue. Leave the parent issue open until the whole change has landed.

If the pull request is waiting on another module, open it as a draft:

```bash
gh pr create --draft --base main --title "feat(app): use the router timeout" --body-file /tmp/pr-body.md
```

### Update after review

```bash
git add router
git commit --only -- router
git push
```

Force-push only when you and the reviewer have agreed to rewrite history. The default is a new commit.

## Commits

- One module per commit, unless the files truly belong together (a workflow file and the script it runs, for example).
- Keep unrelated edits out. A formatting sweep of another module belongs in its own commit.
- Prefer a small commit a reviewer can read in one sitting.
- If a tool writes the message, read it. Change it when it describes the wrong thing.
- A commit on this repository does not publish a module. Publishing is a tag, and it is a maintainer step. See [Releases](#releases).

`nix run .#commit-check` lists modules with uncommitted changes. `nix run .#commit` walks the ones you select.

## Pull requests

### Title

Use a Conventional Commit title:

```text
type(scope): description
```

Types: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`, `chore`, `style`, `revert`.

The scope is the module directory (`router`, `middleware/cors`) or a repository area (`ci`, `nix`, `dotfiles`, `deps`, `docs`, `middleware`, `scripts`, `repo`). You can omit the scope when the title is already specific, as in `docs: clarify contribution workflow`.

Examples:

- `feat(router): add configurable timeout`
- `fix(binding): handle invalid values`
- `refactor(app): simplify startup lifecycle`
- `chore(app): update router dependency`
- `docs: clarify contribution workflow`

You choose the final title. A suggestion from a tool is only a starting point.

### Body

The template asks for:

- **Summary.** What changed, and why.
- **Related Issue.** `Refs #123`, or `No related issue (maintenance).` for a maintenance change with no issue.
- **Module scope.** The primary module, any extra modules, and whether the change breaks callers.
- **Dependencies.** Other pull requests or released tags this one needs. `None` when there are none.
- **Testing.** What you ran and what happened.
- **Checklist.**

A pull request that touches a second module lists it under **Additional modules** and says why those files are in this pull request.

### One issue, several pull requests

Issue #123 can need a router change and an app change. Open two pull requests, both with `Refs #123`. Do not put `Fixes #123` on the first one. GitHub would close the issue when that pull request merges, before the app change exists.

Sub-issues are for tracking, and only when they help. They are not created for every module.

```text
Issue #123  Add a configurable timeout
  Sub-issue #124  Router timeout support
  Sub-issue #125  App exposes the timeout
```

The router pull request says `Refs #123` and, if it finishes that sub-issue, `Closes #124`. The parent stays open until you close it.

### Order

Independent modules can proceed at the same time. Binding and logging can have two open pull requests.

When app code calls new router behavior:

1. Merge and, when callers outside this repo need it, tag `router`.
2. Point `app` at that tag.
3. Open or mark the app pull request ready.

Until that tag exists, keep the app pull request as a draft. Passing tests inside this workspace is not enough: `go.work` and `replace` lines compile the local checkout, not the released module.

### Review

CI runs on every pull request, including pull requests from forks. Fix failures by pushing to the same branch. The [PR Policy](#tests-and-quality-checks) check also runs when you edit the title or the body.

Maintainers merge after the required checks are green and the review is done. Merging updates the default branch. It does not create a module tag.

## Tests and quality checks

CI runs these on pull requests and on pushes to `main`:

| Check | Command | Required to pass |
| --- | --- | --- |
| Flake structure | `nix flake check --no-build` | yes |
| Formatting | `nix run .#fmt-check` | yes |
| Module tidy | `nix run .#tidy` | yes, and the tree must be clean afterward |
| Lint | `nix run .#lint` | yes |
| Unit tests with the race detector | `nix run .#test-race` | yes |
| Integration tests | `nix run .#test-integration` | yes |
| Pull request title and body | PR Policy workflow | yes for the rules below |
| Router benchmarks | benchmark workflow, router changes only | yes when that workflow fails |

`nix run .#lint-soft` reports suggestions. A failure there does not fail CI.

Warnings from PR Policy do not fail the check. They cover extra modules, a multi-module pull request with no explanation, a dependency that is still open, and a `go.mod` edit that does not name a dependency. Read them. Fix them when they match the change.

PR Policy fails the check when:

- the title is not `type(scope): description` or `type: description` with a known type
- the scope is not a real module or repository area
- **Summary** is empty
- **Related Issue** has no issue number and is not `No related issue (maintenance).`
- **Primary module** is empty or unknown
- a required placeholder such as `Refs #` is still blank

The policy job uses `pull_request` with read-only permissions. It does not run with a privileged token, so a pull request from a fork is checked the same way as one from a branch in this repository.

Maintainers can require the check in a GitHub ruleset. The check name is `PR Policy`. This repository's workflows do not change branch protection or rulesets. Add the check in the GitHub settings if merges should wait for it.

### Manual equivalents

Format and lint need `golangci-lint` and the config files in the repo root. For one module:

```bash
golangci-lint fmt --diff --config .golangci.yaml router/...
golangci-lint run --config .golangci.yaml router/...
go mod tidy -C router
go test -C router ./... -race -count=1
go test -C router ./... -tags=integration -race -count=1
```

Repeat for each module you changed. To list publishable modules the way CI does, skip `examples`, `example`, and run the commands in each directory that contains a `go.mod` you touched. `nix run .#test` walks the modules for you and is what CI matches, including the race detector on `nix run .#test-race`.

Changes to the helper scripts have their own tests:

```bash
bash scripts/contrib/tests/run.sh
```

## Dependencies across modules

A module's `go.mod` records the released version it builds against. Inside this repository, `replace` lines point those requirements at the neighboring directory, and `go.work` does the same for local commands. Both are local conveniences.

That split has practical consequences:

- Tests here can pass while the released module does not contain your new API.
- `GOWORK=off` turns off `go.work`. It does not remove `replace` lines in `go.mod`.
- A consumer outside the repository only sees a tag on the module proxy.

Before you treat a dependent pull request as ready:

1. The dependency pull request is merged.
2. The module has a tag, such as `router/v0.16.0`, and that tag contains the merge.
3. The consumer's `require` line names that version.

Check the tag from outside the workspace:

```bash
dir=$(mktemp -d)
cd "$dir"
go mod init example.com/consume-router
go get rivaas.dev/router@v0.16.0
```

If `go get` cannot fetch the version, the tag is not published. Keep the consumer pull request in draft.

Release the modules a consumer needs first. Router and binding can be tagged in either order when they do not depend on each other. Tag both before the app module that requires the new versions.

## Releases

Releases are for maintainers. Contributors do not need to tag a module for a pull request to be reviewed or merged.

A release is an annotated Git tag on the default branch:

```text
router/v0.16.0
```

The version is a semantic version, `vMAJOR.MINOR.PATCH`. `nix run .#release` offers a patch, minor, major, or custom number. You choose the bump. The tool does not infer it from commit text. The tag message starts with the module and the version:

```text
router v0.16.0

- add configurable timeout
```

See what is unreleased:

```bash
nix run .#release-check
```

Create tags interactively. The command asks before it tags, and it asks again before it pushes:

```bash
nix run .#release
```

It does not run as part of `nix run .#pr`. Merging a pull request does not run it either.

### Manual release

Fetch tags, read the commits since the last `module/v*` tag, pick the next version, then tag and push that tag only:

```bash
git fetch origin --tags
git log router/v0.15.1..HEAD -- router
git tag -a router/v0.16.0 -m "router v0.16.0"
git push origin router/v0.16.0
```

Confirm the tag on the remote:

```bash
git ls-remote --tags origin 'router/v0.16.0'
```

Then update modules that should require the new version, in a separate commit and pull request.

## Troubleshooting

**The working tree has changes in several modules.**
That is normal. Select one module. With Nix, `prepare` copies it into a worktree and leaves the rest in place. By hand, stage and commit only that module's paths.

**Something is already staged.**
`nix run .#commit` commits the selected module with `git commit --only`. Other staged files remain staged. By hand, run `git commit --only -- <module>` after you have checked `git status`.

**The branch already exists.**
`prepare` reuses a branch that already has commits and does not copy files onto it. Pick another name, or adopt that branch if it is the work you want.

**The pull request already exists.**
`create` prints the existing URL and stops. Update the open pull request by pushing new commits.

**`gh` says you are not logged in.**
Run `gh auth login` and retry. You can also open the pull request in the browser. The template and the policy check are the same.

**Nix or Cursor is not installed.**
Use the [manual workflow](#manual-workflow). CI does not look at how the branch was prepared.

**The dependency is merged and the consumer tests fail outside the workspace.**
The tag may be missing, or `go.mod` may still describe an older version. Follow [Dependencies across modules](#dependencies-across-modules). A green `go test` in this repository can still be compiling the local `replace` checkout.

**CI failed.**
Open the failing job. Formatting and tidy failures usually mean you should run `nix run .#fmt` or `nix run .#tidy` and commit the result. A policy failure names the title or body line to edit. Push the fix to the same branch.

**The branch conflicts with the default branch.**
Update your branch from the default branch, fix the conflicts in the module you own, and push. Do not merge unrelated modules into the branch to make the conflict go away.

**`prepare` stopped halfway.**
The original checkout is left unchanged. Plans are stored under the Git common directory in `rivaas/pr-state.json`, not on the GitHub issue. Run `nix run .#pr -- status 123`. If a worktree is half-updated, remove that worktree with `git worktree remove` and run prepare again. The helper will not reset your original tree.

**You need to close the parent issue.**
`status` tells you when every recorded plan is merged or already on the default branch. Close the issue yourself after you have checked that the feature is actually complete. Sub-issues closing does not close the parent.
