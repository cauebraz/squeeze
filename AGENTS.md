# AGENTS.md

squeeze compacts tool output for coding agents. Docs: `README.md`, `docs/cli.md`, `docs/adding-a-format.md`, `CONTRIBUTING.md`.

## Commands

```
just check        # lint and test
just cover        # fails below 95 percent
just golden       # rewrite golden files after an intended output change
```

Go 1.27. Linters run through `go tool`.

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

`core`, `sanitize` and `logscan` import no format. A format package exports only `Format`. Formats are registered in `DefaultRegistry`.

## Invariants

- No sensitive value reaches stdout or `--output json`.
- Output is plain text, deterministic, ends with a newline. Every cut states a count and the flag that shows the items.
- Errors and warnings are never dropped.
- stdout is empty unless the exit code is 0 or 3. Exit codes: 0, 1, 2, 3, 64.
- Follow the tool's own semantics and cover them with a fixture.
- Fixtures come from public, permissively licensed sources. Terraform's testdata is BUSL-1.1; do not copy it.
- Standard library only outside tests. `go-cmp` is the one test dependency.
- A behavior change updates `docs/cli.md` or `docs/formats/<name>.md`.
