# AGENTS.md

squeeze compacts tool output for coding agents. User contract: `README.md`, `docs/cli.md`. New format: `docs/adding-a-format.md`. Commits and pull requests: `CONTRIBUTING.md`.

## Commands

```
just check        # before the first edit and after the last
just cover        # fails below 95 percent
just commits      # commit messages and branch name since origin/main
just check-each   # just check on every commit since origin/main
```

Go 1.27. Linters run through `go tool`; nothing to install.

## Layout

```
cmd/squeeze/              main
internal/cli/             subcommands, flags, exit codes
internal/core/            Format interface, Registry, Result, JSON envelope
internal/sanitize/        control characters and escape sequences
internal/logscan/         error and warning lines
internal/tfdiag/          diagnostics with file and line
internal/jsonl/           line-delimited JSON
internal/formattest/      contract suite every format runs
internal/formats/<name>/  one package per format, testdata/<case>/ with golden files
docs/formats/<name>.md    one page per format
```

`core`, `sanitize` and `logscan` import no format. A format package exports only `Format`. Formats are registered in `DefaultRegistry` and nowhere else.

## Rules

- No sensitive value reaches stdout or `--output json`.
- Output is plain text, deterministic, ends with a newline. Every cut states a count and the flag that shows the items.
- Errors and warnings are never dropped.
- stdout is empty unless the exit code is 0 or 3. Exit codes: 0, 1, 2, 3, 64.
- Follow the tool's own semantics; cite its source in the PR and add a fixture.
- A golden file change is a behavior change: `just golden`, read the diff, justify each line in the PR.
- Fixtures come from public, permissively licensed sources. Terraform's testdata is BUSL-1.1; do not copy it.
- Standard library only outside tests. `go-cmp` is the one test dependency.
- Fixed sets of values are named types with constants; `switch` lists every value.
- Tests compare structures with `cmp.Diff`. A test that guards an invariant fails when the invariant is broken.
- No comments in code. Plain ASCII everywhere.
- A behavior change updates `README.md`, `docs/cli.md` or `docs/formats/<name>.md` in the same commit.
- One logical change per commit, each passing `just check`. Pull requests near 200 lines outside `testdata/`.
- Without committing, list the proposed commits with their files and messages.
