# Adding a format

A format is one package under `internal/formats/<name>/` that implements `core.Format`:

```go
type Format interface {
	Name() string
	Description() string
	Detect(head []byte) bool
	Configure(fs *flag.FlagSet) Squeezer
}
```

- `Name` is the subcommand: lowercase letters, digits and hyphens, starting with a letter.
- `Description` is one line for `squeeze -h`.
- `Detect` gets the first 64 KiB of the input and answers from markers the tool always prints. It must reject input of every other registered format.
- `Configure` registers the format's own flags on `fs` and returns the function that squeezes the input. Auto-detection calls it with an empty flag set, so the defaults must suit an agent that passes no flags.

The returned `core.Result` holds `Text`, the output for the agent, and `Data`, any JSON-serializable value that `--output json` places under `data`.

## Test layout

Each case is a directory under `internal/formats/<name>/testdata/`:

```
testdata/<case>/input.<ext>   the tool's output; a symlink to another case's input is fine
testdata/<case>/args          optional: format flags for this case, separated by whitespace
testdata/<case>/want.txt      golden text output, written by `just golden`
testdata/<case>/want.json     golden --output json envelope, written by `just golden`
```

A flag variant is a new case with an `args` file and a symlinked input, so every flag combination that matters has a golden file.

## Checklist

1. Create `internal/formats/<name>/` with the format and its unit tests. Use `github.com/google/go-cmp` for structural comparisons in tests.
2. Add one case directory per scenario, from the tool's real output or its published schema. Never use private or employer material, and never copy test data from a source whose license is not compatible with MIT.
3. Add a contract test; it checks detection, determinism, plain text, the JSON envelope and the golden files of every case:

   ```go
   func TestContract(t *testing.T) {
   	formattest.Contract(t, Format{}, "testdata")
   }
   ```

4. Add `FuzzSqueeze`, seeded with `formattest.Inputs`; `just fuzz-all` fails for a format without it. Assert properties in the fuzz body, not only the absence of a crash.
5. If the tool marks secrets, add a test that extracts every marked value from the input and asserts none appears in the text or the envelope.
6. Register the format in `DefaultRegistry` in `internal/cli/cli.go`. Order matters only when two formats could detect the same input; a test fails when a case is detected by any other format.
7. Run `just golden`, read every file it wrote, then `just check` and `just cover`.
8. Add `docs/formats/<name>.md` (input, detection, output, flags, JSON data, limitations) and a row in the README's format table.

Real tool output that must stay private goes in `internal/formats/<name>/testdata/real/`, which git ignores. `formattest.Real` runs over every file there that the format detects, or over the directory named by `SQUEEZE_REAL_DIR`.
