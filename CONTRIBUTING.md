# Contributing

squeeze takes changes as small pull requests made of micro commits with Conventional Commits messages, merged by rebase onto `main`. Every commit on `main` builds, passes `just check` and describes one change, so `git log`, `git bisect`, `git revert` and the release changelog work commit by commit.

`README.md` and `docs/cli.md` are the user contract and `docs/adding-a-format.md` the guide for a new format. `AGENTS.md` holds the code rules (output invariants, fixtures, tests, no comments in code); they apply to human contributors too.

## Before you start

- Open an issue first for a new format, a new flag, or any change to the `--output json` envelope or the exit codes. A fix or a change inside one format can go straight to a pull request.
- Install Go 1.27 or later and `just`. Linters run through `go tool`; nothing else to install.
- Run `just check` on a clean checkout before the first edit, so a failure after your change is yours and not the baseline's.

## Workflow

1. Update `main` and create a branch: `git switch -c <user>/<type>-<description> origin/main`.
2. Make the change as a series of micro commits, each one passing `just check`.
3. Before you push, run `just check`, `just cover`, `just commits` and `just check-each`.
4. Open a pull request. Keep it in draft while it is not ready for review.
5. Address review, rebase on `main`, and the maintainer merges with "Rebase and merge".

## Branches

A branch is short-lived: one person, one concern, merged or closed within two days of the first commit. Longer work is split into several pull requests rather than kept on one branch.

The name is `<user>/<type>-<description>`:

- `<user>` is your GitHub login, lowercase.
- `<type>` is the commit type of the main change (see below).
- `<description>` is lowercase words joined by hyphens.

Examples: `cauebraz/feat-trivy-format`, `alice/fix-pytest-fold-count`, `bob/docs-contributing`. `just commits` checks the shape; CI checks it on every pull request.

Keep the branch current by rebasing, not by merging `main` into it:

```
git fetch origin
git rebase origin/main
git push --force-with-lease
```

`main` accepts no merge commits, so a branch that contains one fails `just commits`.

## Micro commits

A micro commit is the smallest change that stands on its own. Each commit:

- does one thing, which its subject states without "and";
- builds and passes `just check` by itself; CI runs `just check-each`, which checks out every commit of the pull request and runs `just check` on it;
- carries the tests that prove its behavior, the golden files its code changes (`just golden`) and the docs that describe it (`README.md`, `docs/cli.md`, `docs/formats/<name>.md`);
- keeps refactoring, renames and moves apart from behavior changes; a refactor commit leaves every golden file unchanged.

A typical order inside one pull request:

```
refactor(pytest): extract the failure key from the summary parser
test(pytest): add a fixture with failures at the same line
feat(pytest): fold failures that fail at the same line
```

Fix earlier commits of your branch with fixup commits rather than a new "address review" commit:

```
git commit --fixup <sha>
git rebase -i --autosquash origin/main
```

During review you may push `fixup!` commits so the reviewer sees only the delta; squash them with `--autosquash` before the merge. `just commits` rejects a `fixup!` subject, so the pull request cannot merge while one remains.

## Commit messages

Messages follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <summary>

<body>

<footers>
```

Subject:

- `<type>` is one of the types below; `<scope>` is optional.
- `<summary>` is imperative ("add", not "added" or "adds"), starts lowercase, has no trailing period and names no issue or pull request.
- The whole subject is at most 72 characters; aim for 50.

Body, optional: separated from the subject by one blank line, wrapped at 72 columns (a URL may run longer), and explains why the change is made and what the reader cannot see in the diff. The diff already shows how.

Footers come last, one per line as `Token: value`: `Refs: #123`, `BREAKING CHANGE: <how to migrate>`, `Co-authored-by: Name <email>`.

Plain ASCII in every message, as in every document of this repository.

### Types

| Type | Use for |
|---|---|
| `feat` | a capability a user sees: a new format, flag, condition or output behavior |
| `fix` | output, exit code or detection that contradicts the README, `docs/cli.md`, a format's docs page or the tool's own semantics |
| `perf` | the same output with less time or memory, with a benchmark in the body |
| `refactor` | code restructured with no behavior change; golden files unchanged |
| `test` | tests or fixtures for existing behavior |
| `docs` | documentation only: `README.md`, `docs/`, `AGENTS.md`, this file |
| `build` | `justfile`, `go.mod` directives, build flags |
| `ci` | `.github/workflows/` |
| `chore` | version bumps and pins, renames that change no behavior, repository settings files |
| `revert` | reverting a commit: `revert: <subject of the reverted commit>`, body `This reverts commit <sha>.` |

A change that fits two types is two commits. When it cannot be split, use the type with the larger effect on users.

### Scopes

The scope names the part of the repository the commit changes: a format (`tfplan`, `tfstate`, `tfjson`, `tfvalidate`, `tflint`, `ghactions`, `buildkit`, `jest`, `junit`, `kubectl`, `npmaudit`, `precommit`, `checkov`, `trivy`, `gotest`, `pytest`, `generic`), a package (`cli`, `core`, `tfdiag`, `jsonl`, `formattest`) or `bench`. Leave the scope out when a commit spans several of them.

### Breaking changes

Until 1.0.0 the text output and a format's `data` may change between commits; that is not a breaking change. A breaking change is one that changes what `docs/cli.md` promises to keep: a key of the `--output json` envelope, the meaning of an exit code, or a removed or renamed subcommand or flag.

A breaking change has both markers: `!` before the colon and a `BREAKING CHANGE:` footer that says how to migrate. For a hypothetical flag rename:

```
feat(cli)!: rename --raw to --raw-dir

BREAKING CHANGE: pass --raw-dir <dir> where --raw <dir> was used.
```

### Examples

```
feat(trivy): add a format for trivy --format json
fix(generic): keep error lines in a cut
test(tfplan): check the list depth limit by behavior, not wall time
refactor(tfdiag): share the diagnostic renderer with tflint
docs: state which cuts are announced and which are fixed limits
ci: run checks on ubuntu only
```

Rejected:

```
Fix bug                              no type, says nothing
feat: Add initial arch proposal      uppercase summary
fix(pytest): folded failures.        past tense, trailing period
feat: add tflint and fix pytest      two changes, two commits
```

## Pull requests

One pull request is one concern: one fix, one format, one refactor. Unrelated cleanups go in their own pull request.

Size:

- Aim for 200 changed lines or fewer outside `testdata/`, and at most about 5 commits. Around 400 lines the reviewer may ask for a split. Review research puts the effective range at 200 to 400 lines per review session; it is a range, not a hard limit.
- Fixtures and golden files under `testdata/` do not count toward the size, but every golden line that changes still needs a reason in the description.
- Deleting files is cheap to review; moving code and changing it is not, so move in one pull request and change in the next.
- `just commits` prints the commit count and the size outside `testdata/`.

Split a large change by layer or by step: a refactor that prepares the code, then the tests, then the behavior; or one format at a time. A pull request that depends on another targets that branch as its base until the first one merges.

Title: the same format as a commit subject. A single-commit pull request uses that commit's subject.

Description: fill in `.github/pull_request_template.md`. It asks for the problem, the change, the evidence (tool source or docs for a tool semantic, each changed golden line), the docs updated and the checks run.

Review: CI runs `just check`, `just cover`, `just vuln` and `just fuzz-all` on Go 1.27, `just test` on macOS, the Terraform conformance suite, and, on pull requests, `just commits` and `just check-each`. All of them pass before a merge.

Merge: "Rebase and merge" only. Every commit of the branch lands on `main` as it is, which is why each one passes the checks and carries a Conventional Commits message. The branch is deleted after the merge.

## Checks

`just` lists every recipe. Four of them gate a merge:

```
just check          # goimports, go vet, staticcheck, exhaustive, revive, cross vet, go mod tidy, tests
just cover          # fails below 95 percent coverage
just commits        # commit messages and branch name since origin/main; prints the size
just check-each     # just check on every commit since origin/main, in a temporary worktree
```

`just commits` and `just check-each` take another base as an argument, for example `just commits origin/alice/feat-x` for a stacked branch.

## Coding agents

Agents follow `AGENTS.md`, which applies these rules to agent work. An agent that does not commit itself reports its change as the list of micro commits it proposes, each with its files and message.

## Maintainers

The repository settings enforce what the checks cannot:

- Merge button: only "Allow rebase merging" enabled; "Automatically delete head branches" enabled.
- A ruleset on `main`: require a pull request, require the status checks `check`, `commits` and `conformance`, require linear history, block force pushes and restrict deletions.

## References

- [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
- [Google engineering practices: small CLs](https://google.github.io/eng-practices/review/developer/small-cls.html) and [CL descriptions](https://google.github.io/eng-practices/review/developer/cl-descriptions.html)
- [Linux kernel: separate your changes](https://www.kernel.org/doc/html/latest/process/submitting-patches.html#separate-your-changes)
- [Git: SubmittingPatches](https://git-scm.com/docs/SubmittingPatches)
- [Trunk-based development: short-lived feature branches](https://trunkbaseddevelopment.com/short-lived-feature-branches/)
- [release-please](https://github.com/googleapis/release-please), which reads these messages once releases start
