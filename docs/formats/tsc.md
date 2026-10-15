# tsc

Input: the diagnostics that the TypeScript compiler prints, from `tsc`, `tsc --noEmit` or `tsc -b`, plain or with `--pretty`. tsc has no JSON output. Plain output, the default when stdout is not a terminal, is one line per diagnostic, `<file>(<line>,<column>): <category> TS<code>: <message>`, followed by the message chain as indented lines (`formatDiagnostic` and `flattenDiagnosticMessageText` in `src/compiler/program.ts` at TypeScript v5.9.3). `--pretty` prints `<file>:<line>:<column> - <category> TS<code>: <message>` with color codes, a code frame, the related information and a `Found N errors` summary (`formatDiagnosticsWithColorAndContext` in `program.ts` and `getErrorSummaryText` in `src/compiler/watch.ts` at v5.9.3). A diagnostic without a file, such as an unknown command-line option, prints as `<category> TS<code>: <message>`.

Detected when the first line that is not blank, an `npm run` banner (`> ...`) or a `tsc -b --verbose` status line (`<time> - ...` or `[<time>] ...`, with the `* <project>` list under it) is a diagnostic in one of the three shapes above; a `tsc -b --verbose` log with no diagnostic is detected by its `Projects in this build:` line, and a watch log by `Starting compilation in watch mode`.

```
npx tsc --noEmit | squeeze
npx tsc -b --pretty | squeeze --fail-on error
npm run typecheck 2>&1 | squeeze
```

tsc's plain output is already one line per diagnostic, so squeeze adds a header, sorting, folding and a limit, and the text is about the size of the input (1,151 bytes from the 1,131-byte `plain` fixture). With `--pretty`, the code frames, color codes and the per-file table go: the text is 45 to 47 percent of the input on the `pretty` and `build-pretty` fixtures, where `generic` prints 2,319 bytes of the 3,081-byte `pretty` input against 1,392. The fixtures are captured from TypeScript 5.9.3 and 7.0.2 on a scratch project; both print the same plain output.

## Output

```
$ squeeze < internal/formats/tsc/testdata/pretty/input.txt
tsc: 9 errors in 3 files
error: Type 'string' is not assignable to type 'number'. (src/cart.ts:10:7, TS2322)
error: Type 'string' is not assignable to type 'number'. (src/cart.ts:17:64, TS2322)
  | src/cart.ts:6:25: The expected type comes from property 'weight' which is declared here on type '{ name: string; weight: number; }'
...
error: Type '(event: { id: string; payload: { items: string[]; }; }) => void' is not assignable to type 'Handler'. (src/order.ts:6:7, TS2322)
  Types of parameters 'event' and 'event' are incompatible. Type '{ id: number; payload: { items:
  Item[]; }; }' is not assignable to type '{ id: string; payload: { items: string[]; }; }'. Types of
  property 'id' are incompatible. Type 'number' is not assignable to type 'string'.
```

- The header counts errors, warnings and messages (tsc's `DiagnosticCategory`: `error`, `warning`, `message`, and `suggestion`, counted as a message) and the files that hold them.
- When the input holds tsc's own summary (`Found 9 errors in 3 files.`, `Found 1 error in <file>:<line>`, `Found N errors in the same file, starting at: ...`, `Found N errors.`), squeeze compares it with the errors it read: tsc counts only diagnostics of category `error`, and the files that hold them. A difference, as in a log cut short, prints in the header: `tsc: 8 errors in 3 files; tsc's summary differs: Found 9 errors in 3 files.`
- Each diagnostic prints as `<category>: <message> (<file>:<line>:<column>, TS<code>)`, through the same renderer as `tfvalidate` and `eslint`. The message chain prints under it, wrapped; the related information of `--pretty` prints as `| <file>:<line>:<column>: <message>` lines.
- Diagnostics are sorted errors first, then warnings and messages, and by file, line and column; one without a file sorts first. Identical diagnostics print once with `(xN)`.
- Past `--max-errors`, the notice names the codes of the cut diagnostics with their counts: `... 6 more errors, TS2322 3, TS2307 1, TS2339 1, TS7006 1 (--max-errors 0)`.
- A line that is not a diagnostic, its chain, its code frame or the summary is counted in `... N other lines (--all)`: the `npm run` banner and the `tsc -b --verbose` status lines. Such a line with an error or warning word that is not a status line, such as `npm error ...`, prints as an `error` or `warning` without a code instead, so it is never dropped.
- Color codes and other control characters are removed before the lines are read.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also print the lines that are not diagnostics, such as `tsc -b --verbose` status lines |
| `--max-errors` | `50` | diagnostics listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | a diagnostic has category `error`, or a line outside the diagnostics has an error word |
| `fail` | any tsc diagnostic, or a summary with errors: tsc exits with 1 or 2 when it reports any diagnostic (`emitFilesAndReportErrorsAndGetExitStatus` in `src/compiler/watch.ts` at v5.9.3) |
| `warning` | a diagnostic has category `warning`, or a line outside the diagnostics has a warning word |

## JSON data

`data` holds `tool` (`tsc`), `tool_version` (empty: tsc prints none with its diagnostics), `errors`, `warnings`, `messages`, `files`, `summary` (tsc's `Found ...` line, empty when absent), `summary_errors`, `summary_files` (0 when the summary names no file count), `summary_mismatch`, `codes` (`name`, `count` for every code, most diagnostics first), `diagnostics`, `errors_omitted`, `omitted_by_code` (`name`, `count` for the diagnostics past `--max-errors`; `no code` for a line without one) and `other` (the lines counted in the `--all` notice). Each diagnostic has `severity`, `summary`, `address` (the code, such as `TS2322`) and, when set, `detail` (the message chain, one level per line, whole), `values` (the related information), `file`, `line`, `column` and `count`.

## Limitations

- The message chain prints on at most 3 lines of 100 characters, ending with `...` when longer; `--output json` keeps it whole in `detail`. The related information prints on one line of at most 200 characters.
- tsc 5.9.3 defines no diagnostic of category `warning`; the `categories` fixture, with a warning and a message, is written from `formatDiagnostic`, not captured.
- The code frame of `--pretty` is dropped. A related-information message that starts with a number and a space reads as a code frame line and is dropped too.
- `tsc --watch` is read line by line like any other log: every cycle's diagnostics add up, and the last summary wins.
- A summary in another language (`--locale`) is not recognized and is counted in the other lines; status lines are recognized only with a `h:mm:ss` time, with or without `AM` or `PM`.
