# generic

Input: any text output. squeeze uses it when no other format detects the input, and prints `squeeze: no format detected; used generic` on stderr. Detected when the first 64 KiB hold something other than whitespace, no NUL byte and valid UTF-8, or when they are all whitespace and more follows. Empty or whitespace-only input prints `no output: the command printed nothing` and exits with code 0, as for every format; other input that is not text exits with code 2.

```
npm test 2>&1 | squeeze
docker build . 2>&1 | squeeze generic --max-lines 400
```

The format knows nothing about the tool, so it keeps the text and removes what a terminal would hide or repeat. On the fixtures with color codes, repeats or cuts the text is 17 to 52 percent of the input; text with none of those passes through at the same size or smaller, and input with nothing printable gets a 20-byte notice.

## Output

```
$ squeeze generic < internal/formats/generic/testdata/repeated/input.txt
   Compiling crate-001 v0.1.0
... 38 similar lines (--no-fold)
   Compiling crate-040 v0.40.0
warning: unused variable `x`
... 4 more identical lines (--no-fold)
error: linking with `cc` failed: exit status: 1
... 2 more identical lines (--no-fold)
  note: ld: library not found for -lssl
    Finished in 12.3s

$ squeeze generic --max-lines 8 < internal/formats/generic/testdata/long-log/input.txt
alpha alpha handled by alpha worker
bravo hotel handled by november worker
... 248 lines cut (--max-lines 0)
ERROR db: timeout after 30s on query users
... 49 lines cut, 1 with error or warning words (--max-lines 0)
panic: runtime error: index out of range
... 196 lines cut (--max-lines 0)
romeo tango handled by bravo worker
sierra golf handled by oscar worker
tango november handled by hotel worker
exit status 2
```

- Terminal escape sequences are removed: color codes, cursor movement, terminal titles, hyperlinks (the link text stays), device control and application strings, in their 7-bit and 8-bit forms. A string sequence without its terminator runs to the end of the line. Other control characters, the byte order mark, bidirectional controls and Unicode line separators are dropped, and a tab becomes one space, so a tab-separated table or a tab-indented stack trace prints no wider than the input. Indentation is kept, one space per tab; trailing spaces are not.
- A line rewritten with carriage returns, such as a progress bar, keeps only its last frame with printable text.
- Leading and trailing blank lines are dropped and a run of blank lines becomes one.
- Three or more identical lines in a row print once, followed by `... N more identical lines (--no-fold)`.
- Three or more lines in a row that differ only in their numbers print the first and the last with `... N similar lines (--no-fold)` between them, and the notice counts the hidden lines that carry a warning word. Lines with an error word fold only when identical.
- Error or warning words are `error`, `errors`, `fail`, `fails`, `failed`, `failure`, `failures`, `fatal`, `panic`, `panics`, `panicked`, `exception`, `exceptions`, `traceback`, `tracebacks`, `warn`, `warns`, `warned`, `warning` and `warnings`, as whole words in any case, unless the count before them is zero or the word is negated: `0 warnings suppressed`, `no errors`, `without warnings`, `errors: 0` and `non-fatal` do not count. An exception name also counts as an error word: a word that starts with a capital letter and ends in `Error` or `Exception`, followed by `:` or the end of the line, such as `ValueError: bad`, `E   AssertionError` or `java.lang.IllegalStateException: closed`; `handleError(ctx)` and `onError` do not count.
- Past `--max-lines`, squeeze keeps the first quarter and the last half of the lines, and fills the remaining quarter with lines from the middle that carry an error word, then with lines that carry a warning word, then with the line just before each kept error line, such as the file name above a linter's error or the code line above an exception, in input order. A folded run stays whole: its line and its fold notice are kept or cut together. Each gap prints `... N lines cut (--max-lines 0)` and how many of those lines carry such a word. Fold and cut notices do not count toward the limit.
- A line longer than `--max-line-bytes` is cut at a character boundary, keeping at least one character, and ends with `... N more bytes (--max-line-bytes 0)`.
- Input that is one JSON document on a single line longer than `--max-line-bytes`, such as a pytest-json-report file, is first split with one value or key per line and no indentation, so the cut keeps whole values instead of the first 512 bytes. A document that fits in `--max-lines` after the split prints about 10 percent larger than the input; a larger one is cut like any text. JSON on several lines, several documents and invalid JSON are read as they are.
- Input with no printable text prints `(no printable text)`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-line-bytes` | `512` | bytes per line, `0` for all |
| `--max-lines` | `200` | lines listed, `0` for all |
| `--no-fold` | off | print repeated and similar lines |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a line holds an error word: `error`, `fail`, `failed`, `failure`, `fatal`, `panic`, `exception` or `traceback`, with their plurals, or an exception name such as `ValueError:` |
| `warning` | a line holds `warn`, `warns`, `warned`, `warning` or `warnings` |

Both are word matches on the input, not the tool's verdict: a line such as `retrying after error` sets `error` even when the run succeeded. Name the tool's format when one exists for an exact condition.

## JSON data

`data` holds `tool`, always `text` since no tool is known, and counts of input lines: `lines`, `printed`, `blank_dropped`, `folded`, `lines_omitted`, `flagged_lines_omitted` (omitted lines with an error or warning word), `long_lines` (printed lines cut by `--max-line-bytes`), `error_lines` and `warning_lines` (lines with an error or a warning word, counted before folding and cutting), and `json_indented` (the input was a one-line JSON document split as above; the line counts are then of the split text). `printed + blank_dropped + folded + lines_omitted` equals `lines`.

## Limitations

- The format does not parse anything: a failure the tool reports without one of the words above can fall in a cut gap. Name the tool's format when one exists, or raise `--max-lines`.
- Input in a legacy encoding such as Latin-1 is not detected; `squeeze generic` accepts it and replaces invalid bytes.
