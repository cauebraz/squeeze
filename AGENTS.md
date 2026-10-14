# Agent guidelines

squeeze compacts a tool's output for coding agents. Read `README.md` for the user contract and `docs/adding-a-format.md` before touching a format.

## Commands

Use `just` for everything; `just --list` shows the recipes.

```
just check          # gofmt, go vet, staticcheck, exhaustive, go mod tidy, tests: run before the first edit and after the last
just cover          # coverage across packages, fails below 95 percent
just golden         # rewrite golden files after an intended output change
just fuzz tfplan 30 # fuzz one format
just fuzz-all 20    # fuzz every format; fails for a format without FuzzSqueeze
just bench          # benchmarks with the output/input ratio (tfplan only)
just conformance    # the Terraform formats against Terraform's command testdata at a pinned commit, in a temp dir
just commits        # commit messages and branch name since origin/main; prints the size outside testdata
just check-each     # just check on every commit since origin/main, in a temporary worktree
```

Linters are pinned with the `tool` directive in `go.mod` and run through `go tool`; do not install them globally. The module supports Go 1.26 and 1.27; CI runs both with `GOTOOLCHAIN=local`.

A change is done when `just check` and `just cover` pass after the last edit.

## Layout

```
cmd/squeeze/            main: wires os.Args and stdio to internal/cli
internal/cli/           subcommands, common flags, exit codes, --raw-dir
internal/core/          Format interface, Registry, Result, conditions, JSON envelope: knows no tool
internal/tfdiag/        Terraform diagnostics shared by tfvalidate, tfjson, tflint and eslint
internal/jsonl/         line-delimited JSON input shared by tfjson and gotest
internal/formattest/    contract suite every format runs
internal/formats/<name>/ one package per format; testdata/<case>/ holds input, args, want.txt, want.json
docs/formats/<name>.md  user documentation of one format
scripts/                check-commits.sh, behind just commits
bench/                  agent measurement: tasks with verifiers and a runner for the Anthropic, OpenAI and OpenRouter APIs; bench/README.md and bench/results.svg hold the last results
```

`internal/core` imports no format. `internal/cli` names a format in one place only, `DefaultRegistry`. Tool knowledge lives under `internal/formats/<name>/` and `docs/formats/<name>.md`.

## Commits, branches and pull requests

`CONTRIBUTING.md` holds the rules; these are the parts an agent applies on every change.

- Plan the change as micro commits before the first edit: one logical change per commit, each passing `just check` by itself, refactors and moves apart from behavior, tests, golden files and docs in the commit whose behavior they cover.
- Messages follow Conventional Commits with the types and scopes in `CONTRIBUTING.md`: subject at most 72 characters, imperative, lowercase summary, no trailing period; a breaking change has `!` and a `BREAKING CHANGE:` footer.
- A branch is `<user>/<type>-<description>`, starts from `origin/main`, holds one concern and is rebased, never merged with `main`.
- A pull request stays near 200 changed lines outside `testdata/` and at most about 5 commits; a larger change is split into pull requests that each pass the checks.
- An agent that does not commit reports the proposed commits in order, each with its files and message, so the user can commit them one by one.
- Before reporting a branch as ready, `just commits` and `just check-each` pass besides `just check` and `just cover`.

## Rules

- Output invariants hold for every format: no secret the tool marks as sensitive reaches stdout; output has no control characters or color codes, is the same for the same input and ends with a newline; items and lines cut are announced with a count and the flag that shows them; a single value cut at a fixed limit is listed on the format's docs page and kept whole in `--output json` when the format's data holds it.
- Errors and warnings are never dropped: a line or record the format does not understand but the tool marks as an error or warning becomes a diagnostic.
- stdout is empty whenever the exit code is not 0 or 3. Exit codes are 0, 1, 2, 3 and 64 as listed in the README; a new failure mode reuses one of them.
- A golden file change is a behavior change. Run `just golden`, read the whole diff, and state in the PR why each changed line is correct.
- Fixtures come from public, permissively licensed sources or are written from the tool's published schema. Terraform's own testdata is BUSL-1.1: run squeeze against it locally if useful, never copy it into this repository. Real or private output goes in `internal/formats/<name>/testdata/real/`, which git ignores.
- A format follows the tool's own semantics where the tool defines them (counts, symbols, which drift to show). Check the tool's source or docs, cite it in the PR, and add a fixture that exercises it.
- A format's parser accepts every value the tool documents. When the tool adds a value (an action, a field), add a fixture that contains it.
- Every `switch` over an enum lists all values; `go tool exhaustive` enforces it.
- Tests compare structures with `cmp.Diff`. Fuzz targets assert properties, not only the absence of a crash. A test that guards an invariant must fail when the invariant is broken; check it once by breaking the code.
- Standard library only in non-test code. `github.com/google/go-cmp` is the test-only dependency; another one needs a reason in the PR.
- No comments in code. Explanations go in the PR description or in `docs/`.
- Documents and messages are plain ASCII.
- A behavior change updates `README.md` or `docs/formats/<name>.md` in the same change.
- There is no `CHANGELOG.md` and none is written by hand. Once public releases start, an automated release tool (semantic-release or release-please) writes it from the commit history.
- `just cover` fails below 95 percent; a change that lowers coverage adds tests before it is done.
- Research notes, plans and drafts stay out of the repository; `notes/` and `research/` are gitignored.
