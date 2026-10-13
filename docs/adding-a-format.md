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

## Checklist

1. Create `internal/formats/<name>/` with the format and its unit tests.
2. Add fixtures to `internal/formats/<name>/testdata/`: one file per scenario, from the tool's real output or its published schema. Never use private or employer material.
3. Add a contract test; it checks detection, determinism, plain text, the JSON envelope and the golden files:

   ```go
   func TestContract(t *testing.T) {
   	formattest.Contract(t, Format{}, "testdata")
   }
   ```

4. Add `FuzzSqueeze`, seeded with the fixtures; `just fuzz-all` fails for a format without it.
5. Register the format in `DefaultRegistry` in `internal/cli/cli.go`. Order matters only when two formats could detect the same input.
6. Run `just golden`, read every file it wrote, then `just check` and `just cover`.
7. Add `docs/formats/<name>.md` (input, detection, output, flags, JSON data, limitations) and a row in the README's format table.

Real tool output that must stay private goes in `internal/formats/<name>/testdata/real/`, which git ignores; `formattest.Real` runs over it when present.
