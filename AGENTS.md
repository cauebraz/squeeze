# Agent guidelines

squeeze compacts a tool's output for coding agents. Read `README.md` for the user contract and `docs/adding-a-format.md` before touching a format.

## Commands

Use `just` for everything; `just --list` shows the recipes.

```
just check          # gofmt, go vet, staticcheck, tests: run before the first edit and after the last
just cover          # coverage across packages, fails below 85 percent
just golden         # rewrite golden files after an intended output change
just fuzz tfplan 30 # fuzz one format
just fuzz-all 20    # fuzz every format; fails for a format without FuzzSqueeze
just bench          # benchmarks with the output/input ratio
```

A change is done when `just check` and `just cover` pass after the last edit.

## Layout

```
cmd/squeeze/            main: wires os.Args and stdio to internal/cli
internal/cli/           subcommands, common flags, exit codes, --raw-dir
internal/core/          Format interface, Registry, Result, JSON envelope: knows no tool
internal/formattest/    contract suite every format runs
internal/formats/<name>/ one package per format with its own testdata/ and golden/
docs/formats/<name>.md  user documentation of one format
```

`internal/core` imports no format. `internal/cli` names a format in one place only, `DefaultRegistry`. Tool knowledge lives under `internal/formats/<name>/` and `docs/formats/<name>.md`.

## Rules

- Output invariants hold for every format: no secret the tool marks as sensitive reaches stdout; output has no control characters or color codes, is the same for the same input and ends with a newline; anything cut is announced with a count and the flag that shows it.
- stdout is empty whenever the exit code is not 0. Exit codes are 0, 1, 2 and 64 as listed in the README; a new failure mode reuses one of them.
- A golden file change is a behavior change. Run `just golden`, read the whole diff, and state in the PR why each changed line is correct.
- Fixtures come from public, permissively licensed sources or are written from the tool's published schema. Terraform's own testdata is BUSL-1.1: run squeeze against it locally if useful, never copy it into this repository. Real or private output goes in `internal/formats/<name>/testdata/real/`, which git ignores.
- A format's parser accepts every value the tool documents. When the tool adds a value (an action, a field), add a fixture that contains it.
- Standard library only in non-test code. A test-only dependency needs a reason in the PR.
- No comments in code. Explanations go in the PR description or in `docs/`.
- Documents and messages are plain ASCII.
- A behavior change updates `README.md` or `docs/formats/<name>.md` in the same change.
- Research notes, plans and drafts stay out of the repository; `notes/` and `research/` are gitignored.
