# Contributing

Open an issue before a new format, a new flag, or a change to the `--output json` envelope or the exit codes. A fix inside one format can go straight to a pull request.

## Setup

Go 1.27 and `just`. Linters run through `go tool`; nothing else to install.

## Workflow

```
git switch -c <user>/<type>-<description> origin/main
# small commits, each passing just check
just check && just cover && just commits && just check-each
```

Open the pull request with `.github/pull_request_template.md` filled in. Rebase on `main` instead of merging it; `main` has no merge commits.

## Branches

`<user>/<type>-<description>`: GitHub login, commit type, lowercase words joined by hyphens. Example: `alice/fix-pytest-fold-count`. One concern per branch.

## Commits

Each commit does one thing and passes `just check` on its own. Tests, golden files and docs go in the commit that changes the behavior. Refactors stay apart from behavior changes.

Messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <summary>
```

- Subject at most 72 characters, imperative, lowercase, no trailing period.
- Type: `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `build`, `ci`, `chore`, `revert`.
- Scope, optional: a format (`tfplan`, `pytest`, ...), a package (`cli`, `core`, `tfdiag`, `jsonl`, `formattest`) or `bench`.
- A breaking change (envelope key, exit code meaning, removed or renamed flag or subcommand) has `!` after the scope and a `BREAKING CHANGE:` footer. Text output and a format's `data` may change freely before 1.0.

Examples:

```
feat(trivy): add a format for trivy --format json
fix(generic): keep error lines in a cut
refactor(tfdiag): share the diagnostic renderer with tflint
docs: state which cuts are announced and which are fixed limits
```

Address review with fixup commits and autosquash them before merge; `just commits` rejects a `fixup!` subject.

## Pull requests

- One concern per pull request.
- Aim for 200 changed lines outside `testdata/` and about 5 commits; split larger work.
- Title in commit subject format.
- Every changed golden line gets a reason in the description.
- Merged with "Rebase and merge" only.

## Checks

```
just check        # goimports, vet, staticcheck, exhaustive, revive, cross vet, tidy, tests
just cover        # fails below 95 percent
just commits      # commit messages and branch name since origin/main
just check-each   # just check on every commit since origin/main
```

CI also runs `just vuln`, `just fuzz-all`, `just test` on macOS and `just conformance`. All pass before a merge.

## Repository settings

Rebase merging only, head branches deleted after merge. A ruleset on `main` requires a pull request, the `check`, `commits` and `conformance` jobs, linear history, and blocks force pushes.
