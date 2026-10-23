# precommit

Input: what `pre-commit run` prints, with or without `--all-files`, `--verbose`, `--show-diff-on-failure` and color.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Run pre-commit as `pre-commit run --all-files 2>&1 | squeeze`; a hook marked `files modified` changed
  the files itself, so rerun pre-commit before editing them by hand.
```

## Example

```
pre-commit run --all-files 2>&1 | squeeze
pre-commit run --all-files --show-diff-on-failure 2>&1 | squeeze precommit --all
```

pre-commit prints one dotted status line per hook.
For a hook that failed or ran with `--verbose`, it also prints the id, exit code, whether the hook modified files, and its output.
squeeze keeps the failed hooks with their output, counts the passed and skipped ones, and keeps pre-commit's own errors and warnings.
On the `failed` fixture the text is 909 of 1,408 bytes.
On `limits`, where one hook prints 125 lines and repeats one failure 18 times, it is 363 of 5,250 bytes, and `generic` printed all 5,250.
A clean run of eight hooks prints two lines.

- Detection: a line, without a tab, ends in a run of three or more dots and `Passed`, `Failed`, `Skipped` or `(no files to check)Skipped`, and is at least 79 columns wide. pre-commit pads every status line to the same width, at least 80 columns minus one (`pre_commit/commands/run.py`, `_compute_cols` and `_full_msg`, pre-commit 4.6.2).

## Output

```
$ squeeze < internal/formats/precommit/testdata/show-diff/input.txt
pre-commit: 8 hooks, 5 failed, 1 passed, 2 skipped; 3 hooks modified files
fail trailing-whitespace (trim trailing whitespace): exit code 1, files modified
  Fixing pre_commit_hooks/util.py
  Fixing pre_commit_hooks/removed.py
...
fail expand-tabs (expand tabs in markdown): files modified
fail no-todo (forbid TODO markers): exit code 1
  73:# TODO: sort keys
  ...
changed: .pre-commit-hooks.yaml
changed: CHANGELOG.md
...
... 56 diff lines (--all)
... 3 passed or skipped hooks (--all)
```

- The header counts the hooks by status and the hooks that modified files. pre-commit has no version line, so the header names no version.
- A hook is failed when its status line says `Failed`: pre-commit fails a hook that exits non-zero or changes `git diff` (`run.py`, `_run_single_hook`). It prints as `fail <id> (<name>)` with `exit code N` from `- exit code:` and `files modified` from `- files were modified by this hook`, then its output. The id comes from `- hook id:`; a hook printed without one shows its name only.
- A hook's output longer than `--max-lines` is cut to its lines with an error or warning word, each with the line after it, unless the same line was already printed; a hook without such lines keeps its first `--max-lines` lines. Each gap prints as `... N lines cut (--max-lines 0)` where the lines were cut. A hook that prints a progress line per file before its errors therefore shows the errors, not the progress. pre-commit runs a hook in parallel batches of files, so the same failure often repeats; the repeats are cut.
- Passed and skipped hooks are counted in `... N passed or skipped hooks (--all)`. `--all` lists each as `pass <name>`, `skip <name>: no files to check` (no file matched the hook) or `skip <name>: SKIP` (the hook id is in the `SKIP` environment variable), with its output when `--verbose` printed some.
- pre-commit's own `[WARNING]` and `[ERROR]` lines print as `warning:` and `error:`; `[INFO]` lines are dropped. `An error has occurred:`, `An unexpected error has occurred:`, `Interrupted (^C):` and `No hook with id ...` print as `error:`. A hook whose status line ends in one of these messages, because pre-commit stopped while it ran, prints as `unfinished <name>`.
- With `--show-diff-on-failure`, each file in pre-commit's `All changes made by hooks:` diff prints as `changed: <file>`; the diff itself is counted in `... N diff lines (--all)` and `--all` prints it. The three lines before it are dropped.
- Lines outside a hook's output that squeeze does not know, such as `Check the log at <path>`, print under `other output:`, cut the same way. A `- duration:` or `- exit code:` line whose number does not fit a 64-bit value prints there too.
- Color codes are removed, so `--color always` and `--color never` give the same output; tabs print as four spaces.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list passed and skipped hooks with their output, and the diff of --show-diff-on-failure |
| `--max-lines` | `30` | output lines per failed hook, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | pre-commit printed an `[ERROR]` line, `An error has occurred:`, `An unexpected error has occurred:`, `Interrupted (^C):` or `No hook with id` |
| `fail` | a hook failed or did not finish, or `error`: pre-commit exits non-zero in each case |
| `modified` | a hook printed `- files were modified by this hook` |
| `warning` | pre-commit printed a `[WARNING]` line |

## JSON data

`data` holds `tool` (`pre-commit`), `tool_version` (always empty), `hooks_total`, `passed`, `failed`, `skipped`, `unfinished`, `modified` (hooks that modified files), `errors`, `warnings`, `hooks`, `hooks_omitted`, `other`, `other_omitted`, `changed_files`, `diff` and `diff_omitted`. `hooks` holds the printed hooks; each has `name`, `id` (empty when not printed), `status` (`passed`, `failed`, `skipped` or `unfinished`), `no_files`, `exit_code`, `files_modified`, `duration_seconds` (from `--verbose`, 0 otherwise), `output` (the printed lines, cut notices included) and `lines_omitted`. `diff` is empty unless `--all` is set.

## Limitations

- A status line is known by its dots and status word, and every status line of one run has the width of the first one. A hook name that ends in dots loses them.
- pre-commit prints a hook's output between blank lines but marks neither end, so a line pre-commit itself would print, such as `[ERROR]`, inside a hook's output stays in that output.
- A hook that runs a test runner or linter prints its output as text; that tool's format in squeeze does not read it. Run the tool directly and pipe it through `squeeze` for that.
- An input that holds only pre-commit messages, such as `[ERROR] Your pre-commit configuration is unstaged.`, has no status line and goes through `generic`; `squeeze precommit` reads it.
