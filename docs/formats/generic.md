# generic

Input: any text output. squeeze uses it when no other format detects the input, and prints one line on stderr that says so. Detected when the first 64 KiB hold something other than whitespace, no NUL byte and valid UTF-8; other input exits with code 2.

```
npm test 2>&1 | squeeze
docker build . 2>&1 | squeeze generic --max-lines 400
```

The format knows nothing about the tool, so it keeps the text and removes what a terminal would hide or repeat. On the fixtures with color codes, repeats or cuts the text is 2 to 53 percent of the input; text with none of those passes through unchanged, and input with nothing printable gets a 20-byte notice.

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
WARNING retrying query users
... 245 lines cut, 1 with error or warning words (--max-lines 0)
romeo tango handled by bravo worker
sierra golf handled by oscar worker
tango november handled by hotel worker
exit status 2
```

- Color codes, cursor movement, terminal titles and hyperlinks are removed; the hyperlink text stays. Other control characters are dropped and a tab becomes four spaces. Indentation is kept; trailing spaces are not.
- A line rewritten with carriage returns, such as a progress bar, keeps only its last non-blank frame.
- Leading and trailing blank lines are dropped and a run of blank lines becomes one.
- Three or more identical lines in a row print once, followed by `... N more identical lines (--no-fold)`.
- Three or more lines in a row that differ only in their numbers print the first and the last with `... N similar lines (--no-fold)` between them. Lines with an error or warning word fold only when identical.
- Error or warning words are `error`, `fail`, `failed`, `failure`, `fatal`, `panic`, `exception`, `traceback`, `warn` and `warning`, as whole words in any case, with their plurals.
- Past `--max-lines`, squeeze keeps the first quarter and the last half of the lines, and fills the remaining quarter with the first lines from the middle that carry an error or warning word. Each gap prints `... N lines cut (--max-lines 0)` and how many of those lines carry such a word. Fold and cut notices do not count toward the limit.
- A line longer than `--max-line-bytes` is cut at a character boundary and ends with `... [N more bytes, --max-line-bytes 0]`.
- Input with no printable text prints `(no printable text)`.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-lines` | `200` | lines listed, `0` for all |
| `--max-line-bytes` | `512` | bytes per line, `0` for all |
| `--no-fold` | off | print repeated and similar lines |

## Conditions for `--fail-on`

None. Words in free text do not say whether a run failed, so `--fail-on` needs a format that does. On input that no format detects, `--fail-on` exits with code 2 and names the formats; under `squeeze generic` any condition exits with code 64.

## JSON data

`data` holds counts of input lines: `lines`, `printed`, `blank_dropped`, `folded`, `cut`, `cut_flagged` (cut lines with an error or warning word), and `long_lines` (printed lines cut by `--max-line-bytes`). `printed + blank_dropped + folded + cut` equals `lines`.

## Limitations

- The format does not parse anything: a failure the tool reports without one of the words above can fall in a cut gap. Name the tool's format when one exists, or raise `--max-lines`.
- Input in a legacy encoding such as Latin-1 is not detected; `squeeze generic` accepts it and replaces invalid bytes.
