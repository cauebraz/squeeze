# pytest

Input: what pytest prints on the terminal, in its default, `-v` or `-q` form, with any `--tb` style and with or without color. Detected when the first 64 KiB hold the `test session starts` line, a `FAILURES`, `ERRORS`, `warnings summary` or `short test summary info` section line, or the final summary line, such as `1 failed, 2 passed in 0.12s`, with at least one of pytest's outcome words in it (`failed`, `passed`, `skipped`, `deselected`, `xfailed`, `xpassed`, `error`) or `no tests ran`.

```
pytest 2>&1 | squeeze
pytest -x -q 2>&1 | squeeze --fail-on fail
```

pytest already prints a summary, so the saving comes from the parts an agent does not read: progress lines, the session header, the source context of every traceback frame and the captured output. On the fixtures, captured from pytest 9.1.1, the text is 7 to 44 percent of the input; the more tests fail, the closer it gets to pytest's own output.

## Output

```
$ squeeze < internal/formats/pytest/testdata/default/input.txt
pytest 9.1.1: 5 failed, 4 passed, 1 skipped, 1 xfailed, 3 warnings, 1 error in 0.02s
ERROR tests/test_calc.py::test_query (setup)
  > raise RuntimeError("database unavailable")
  E RuntimeError: database unavailable
  tests/conftest.py:6: RuntimeError
  ... 2 traceback lines (--full)
FAILED tests/test_calc.py::test_add_wrong
  > assert result == 5
  E assert 4 == 5
  tests/test_calc.py:13: AssertionError
  ... 3 traceback lines (--full)
  ... 1 captured line in stdout call (--full)
FAILED tests/test_calc.py::test_add_param[2-3-6]
  > assert add(a, b) == want
  E assert 5 == 6
  E  +  where 5 = add(2, 3)
  tests/test_calc.py:18: AssertionError
  ... 3 traceback lines (--full)
FAILED tests/test_calc.py::test_dict
  > assert {"name": "web", "port": 80} == {"name": "web", "port": 443}
  E AssertionError: assert {'name': 'web', 'port': 80} == {'name': 'web', 'port': 443}
  E   Omitting 1 identical items, use -vv to show
  E   Differing items:
  E   {'port': 80} != {'port': 443}
  E   Use -v to get more diff
  tests/test_calc.py:22: AssertionError
  ... 1 traceback line (--full)
FAILED tests/test_calc.py::test_fixed
  [XPASS(strict)] fixed already
FAILED tests/test_calc.py::TestDivision::test_zero
  > assert 1 / 0
  E ZeroDivisionError: division by zero
  tests/test_calc.py:51: ZeroDivisionError
  ... 3 traceback lines (--full)
warning calc.py:9: DeprecationWarning: old_api is deprecated, use add (x3)
```

- The header is pytest's final summary line with its separators removed, after the pytest version from the session header. Without that line, as when the run was killed, the header says `no summary line`.
- Every entry of the `ERRORS` and `FAILURES` sections prints as `ERROR` or `FAILED` with the test's node ID, which `pytest <node id>` reruns. pytest titles those entries with the test's name only; squeeze takes the node ID from the matching `short test summary info` line, so the summary needs its default `f` and `E` report characters. An error names its phase: `(setup)`, `(teardown)` or `(collection)`.
- From a traceback, squeeze keeps the `>` lines (the failing source line of each frame), the `E` lines (the exception and the assertion explanation) and the `path:line:` location lines, with the source line that follows a `--tb=short` location. Other lines, such as the source around the failing line and the local variables, are counted in `... N traceback lines (--full)`, and the captured output sections in `... N captured lines in stdout call and stderr call (--full)`. A traceback with none of those lines, as in `--tb=native` or an `[XPASS(strict)]` entry, prints whole.
- `--full` prints the captured sections under the traceback, as `captured stderr call:` followed by their lines, so a failure whose only explanation is in captured stdout, stderr or logs stays readable. `--output json` always carries them under `captured`.
- A report-looking line inside captured output, such as a test that prints `===== running migrations =====` or `1 failed, 2 passed in 0.50s`, stays in the captured section; only an entry title, one of pytest's section titles or a `=`-framed summary line ends it.
- With `--tb=line`, each failure is the `E` lines followed by its `path:line: message` line; squeeze pairs them with the short summary in order.
- pytest's built-in subtests print as `FAILED <node id> [label] (i=1)`, with the label from the `SUBFAILED` summary line; the parent test's `contains N failed subtests` entry follows.
- Failures of the same kind that fail at the same `path:line` with the same `>` source lines, as the cases of a parametrized test do, print once: the first entry in full, then `... N more failures from tests/test_x.py:23 (--no-fold)`. The folded node IDs are in `--output json` under `same_location_tests`. `--max-tests` counts the entries left after folding. A failure without a location line, as with `--tb=no`, never folds.
- Past `--max-output`, squeeze keeps the last lines of the traceback, of each captured section and of the output outside the sections, where the error is, and counts the earlier ones.
- A failure or error that is only in the short summary, as with `--tb=no`, prints with the summary's message.
- Paths under the session's `rootdir` print relative to it. `-q` prints no session header, so paths stay as pytest printed them.
- The warnings summary prints one line per message with the number of times it was raised. pytest groups the locations of a message raised 10 times or more by file; squeeze adds those counts. A message that spans lines is joined with spaces; the source line pytest prints under it is dropped; a message without a location, such as `PytestConfigWarning`, prints as is.
- The `!!! Interrupted: ... !!!` line and a `!!! stopping after N failures !!!` line print as `interrupted:` and `stopped:`.
- Lines outside the report sections that are not the session header or progress, such as `INTERNALERROR>` lines, print under `output outside the report sections:`. An `INTERNALERROR>` line sets the `internal_error` and `fail` conditions.
- The `PASSED`, `SKIPPED`, `XFAIL` and `XPASS` lines of the short summary, and the sections squeeze does not read (`PASSES`, `slowest durations`, plugin sections such as coverage), are counted, one line each, as `... N short summary lines for passed, skipped, xfailed or xpassed tests (--all)` and `... N lines in section "<title>" (--all)`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | print the short summary lines of passed, skipped, xfailed and xpassed tests and the sections squeeze only counts |
| `--full` | off | print the whole traceback and the captured output of every failure |
| `--max-output` | `20` | lines per failure and lines outside the sections, `0` for all |
| `--max-tests` | `50` | failures and errors listed, `0` for all |
| `--max-warnings` | `10` | warnings listed, `0` for all |
| `--no-fold` | off | print every failure, also one that fails at the same line as an earlier one |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | the summary counts an error, or collection was interrupted |
| `fail` | the summary counts a failure or an error, collection was interrupted, pytest printed an `INTERNALERROR>` line, or a failure is listed |
| `internal_error` | pytest printed an `INTERNALERROR>` line |
| `interrupted` | pytest printed an `Interrupted:` line |
| `no_tests` | the summary line says `no tests ran` or counts no outcome |
| `skip` | the summary counts a skipped test |
| `warning` | the summary counts a warning, or the warnings summary lists one |
| `xfail` | the summary counts an xfailed test |
| `xpass` | the summary counts an xpassed test; a strict xpass counts as failed |

## JSON data

`data` holds `tool`, `tool_version`, `summary_found`, `summary`, `interrupted`, `stopped` and `internal_error` when set, `failures`, `failures_folded`, `failures_omitted`, `warning_messages` (`message`, `count`), `warnings_omitted`, `notes`, `notes_omitted`, `outcomes` (the other short summary lines) and `other_sections` (line counts by section title).

- `summary`: `failed`, `passed`, `skipped`, `deselected`, `xfailed`, `xpassed`, `warnings`, `errors`, `other` (counts of types pytest or a plugin adds, such as `subtests passed` or `rerun`), `duration` and `line`.
- `failures`: `kind` (`failed` or `error`), `test`, `lines` and, when set, `subtest`, `when`, `message`, `hidden` (traceback lines not shown), `captured` (one object per section with `name`, `lines` and `omitted`), `omitted`, `from_summary_only`, `location` (the last `path:line` of the traceback) and `same_location_tests` (the node IDs folded into this entry).

## Limitations

- The format reads terminal text, which pytest does not version. It follows pytest 9.1.1 (`src/_pytest/terminal.py`); a plugin that rewrites the report can change what squeeze sees.
- `-qq` prints no summary line, and an error that stops pytest before the session starts, such as a `conftest.py` that fails to import, prints no session header. Neither is detected, and both go to `generic`, which keeps the error lines.
- The short summary shortens its messages to the terminal width unless pytest runs on CI or with `-vv`; a message that only the summary carries keeps pytest's `...`.
- Under `-q` the final summary line is not `=`-framed, so a test that prints such a line to captured output and runs with `-q` and `-rN` (no short summary) can end its own entry early.
