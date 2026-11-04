# AGENTS.md

squeeze compacts tool output for coding agents. `README.md` and `docs/cli.md` say what users get; `docs/adding-a-format.md` says how a format is built; `CONTRIBUTING.md` covers branches, commits and pull requests. Read them before changing the code they describe.

## Commands

`just` lists every recipe. These four gate a change:

```
just check        # before the first edit and after the last
just cover        # coverage, fails below 95 percent
just commits      # commit messages and branch name since origin/main
just check-each   # just check on every commit since origin/main
```

Linters are pinned in `go.mod` and run through `go tool`; nothing to install. `just fmt` groups imports as standard library, third party, this module. The module needs Go 1.27; CI runs 1.27.2 with `GOTOOLCHAIN=local`.

## Layout

```
cmd/squeeze/              main: os.Args and stdio into internal/cli
internal/cli/             subcommands, common flags, exit codes, --raw-dir
internal/core/            Format interface, Registry, Result, conditions, JSON envelope, report helpers
internal/sanitize/        strips control characters, escape sequences and invisible runes
internal/logscan/         error and warning lines, lines around a JSON document, tail cuts
internal/tfdiag/          diagnostics with file and line; Terraform formats, checkov, trivy
internal/jsonl/           line-delimited JSON; tfjson and gotest
internal/formattest/      contract suite every format runs
internal/formats/<name>/  one package per format; testdata/<case>/ has input, args, want.txt, want.json
docs/cli.md               usage, flags, JSON envelope, exit codes, guarantees
docs/formats/<name>.md    one page per format
scripts/                  check-commits.sh, behind just commits
bench/                    agent measurement; bench/README.md and bench/results.svg hold the last run
```

`internal/core`, `internal/sanitize` and `internal/logscan` import no format. `internal/sanitize` imports nothing from this module. A format package exports `Format` and nothing else; raw input types are named `raw<Thing>`. `internal/cli` lists the formats in `DefaultRegistry` and nowhere else. Everything a format knows about its tool lives in `internal/formats/<name>/` and `docs/formats/<name>.md`.

## Commits

The full rules are in `CONTRIBUTING.md`. The short version:

- Plan the commits before the first edit. One logical change per commit, each passing `just check` on its own.
- Refactors and moves go in their own commits. Tests, golden files and docs go in the commit whose behavior they cover.
- Conventional Commits: subject at most 72 characters, imperative, lowercase, no trailing period. A breaking change has `!` and a `BREAKING CHANGE:` footer.
- Branch `<user>/<type>-<description>` from `origin/main`, one concern, rebased rather than merged.
- A pull request stays near 200 changed lines outside `testdata/` and about 5 commits. Split anything larger.
- If you do not commit, list the commits you propose in order, each with its files and message.
- A branch is ready when `just check`, `just cover`, `just commits` and `just check-each` all pass.

## Output

- No value the tool marks as sensitive reaches stdout or `--output json`.
- Output is plain text: no control characters, no color codes, same output for the same input, newline at the end.
- Every cut says how many items it dropped and which flag shows them. A value cut at a fixed limit is listed on the format's docs page and kept whole in `--output json` when the data has it.
- Errors and warnings are never dropped. A line the format does not understand but the tool marks as an error or warning becomes a diagnostic.
- stdout is empty when the exit code is not 0 or 3. Exit codes are 0, 1, 2, 3 and 64, as in `docs/cli.md`; a new failure mode reuses one of them.
- A golden file change is a behavior change. Run `just golden`, read the whole diff, and say in the PR why each changed line is right.
- Where the tool defines a semantic (counts, symbols, which drift to show), follow the tool. Cite its source or docs in the PR and add a fixture for it.
- The parser accepts every value the tool documents. When the tool adds one, add a fixture that contains it.

## Code

- Standard library only outside tests. `github.com/google/go-cmp` is the one test dependency; another needs a reason in the PR.
- A string with a fixed set of values is a named type with constants, and every `switch` over it lists all of them. `exhaustive` enforces this.
- Tests compare structures with `cmp.Diff`. Fuzz targets assert properties, not only the absence of a crash.
- A test that guards an invariant must fail when the invariant is broken. Break the code once to check.
- Coverage stays at or above 95 percent; a change that lowers it adds tests first.
- Fixtures come from public, permissively licensed sources or are written from the tool's published schema. Terraform's own testdata is BUSL-1.1 and is never copied into this repository.
- No comments in code. Explanations go in the PR description or in `docs/`.
- Plain ASCII in documents and messages.
- A behavior change updates `README.md`, `docs/cli.md` or `docs/formats/<name>.md` in the same change.
- There is no hand-written `CHANGELOG.md`; a release tool will generate one from the commit history.
