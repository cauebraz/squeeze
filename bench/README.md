# Measuring what squeeze saves

`bench/run.py` measures the end-to-end effect of squeeze on a coding agent: the same model solves the same tasks with and without squeeze in the pipe, and the run records what the API billed, what the shell tool returned, how many turns the agent took and whether a verifier accepted the result.

## Method

- A task is a directory under `bench/tasks/<name>/`: a `repo/` with a planted defect, and a `task.json` with the prompt, an optional `setup` command, a `verify` command that exits 0 only when the defect is fixed and nothing else was damaged, a `solution` the self-test applies to prove the verifier works, and the line the agent's `AGENTS.md` gets in each arm.
- The agent is the smallest loop that counts as one: a system prompt made of `AGENTS.md`, the task prompt, and one tool, `shell`, that runs a command in a fresh copy of the repo. It stops when the model answers without a tool call or after `--max-turns`.
- The two arms differ in one line of `AGENTS.md` and in `PATH`. Control runs the tool as is; squeeze runs `tool | squeeze` and has the binary built from this checkout on `PATH`.
- Each arm runs `--runs` times per task, because agents vary between runs. Every run is a line in `bench/results/<timestamp>.jsonl` with `prompt_tokens`, `completion_tokens`, `cached_tokens`, `tool_bytes`, `turns`, `seconds`, `passed` and the commands the agent ran.
- Tool output is capped at 200,000 bytes per call, so the control arm pays for what the tool prints up to that cap, as an agent without squeeze would.

The report prints, per task and arm, the pass rate and the mean tokens, tool bytes and turns, then the token saving per task. Read the pass rate first: a task where the squeeze arm passes less often than control is a defect in that format, and the run's `commands` show what the agent read.

## Tasks

| Task | Tool | Defect | Verifier |
|---|---|---|---|
| `pytest-failures` | pytest | a discount floor at 40 percent instead of 50, and a receipt rounded half-even where the tests expect half-up: 48 of 328 tests fail, with dict diffs that pytest's short summary cuts at the terminal width | every test passes, no test skipped or removed |
| `gotest-race` | go test | `Queue.Push` without the lock, caught by `-race` | `go test -race ./...` passes, the race test still exists |
| `tflint-high` | tflint | an untyped variable, an unused one, an unpinned provider and a deprecated index | `tflint --minimum-failure-severity=warning` exits 0 with the resources intact |
| `lint-log` | a shell script | a 400-line log with one error naming a config key | the script exits 0 with the key fixed |

The tasks need `uv`, `go` and `tflint` on `PATH`.

## Running

```
bench/run.py --self-test                                  # verifiers and solutions, no API calls
OPENROUTER_API_KEY=... just measure anthropic/claude-sonnet-4.5 5
OPENROUTER_API_KEY=... bench/run.py --model <id> --runs 5 --arm squeeze --task pytest-failures
bench/run.py --report bench/results/<timestamp>.jsonl     # print the table again
```

`--model` takes an OpenRouter model id; the model must support tool calling. `--keep` leaves the working copies in the temp directory for inspection. Results are not committed.

## Limitations

- Four tasks, each small: the number says whether the effect exists, not its size on real work.
- One agent loop and one prompt. A product agent has its own caps, retries and context handling; the saving there depends on where its caps sit.
- Token counts come from the API's `usage` object, so caching and pricing differences between models change the dollar figure but not the byte and turn counts.
