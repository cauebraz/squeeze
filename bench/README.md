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
OPENROUTER_API_KEY=... just measure anthropic/claude-opus-5.5 10 --jobs 10
bench/run.py --report bench/results/<timestamp>.jsonl     # print the table again
```

`--model` takes an OpenRouter model id; the model must support tool calling. `--jobs` sets how many runs are in flight at once; every run has its own working copy and the API is the bottleneck, so ten jobs finish about ten times sooner until the provider's rate limit answers 429, which the runner retries five times with backoff. Rows land in the results file as runs finish, so `run` and `task` in each row, not the line order, identify a run. `--keep` leaves the working copies in the temp directory for inspection. Results are not committed.

## Measured

Run on 2026-10-14 with `anthropic/claude-opus-5.5` through OpenRouter, 5 runs per cell, `--jobs 10`, squeeze at the commit that folds pytest failures by location and folds similar warning lines in `generic`. All 40 runs passed the verifier. Tokens are prompt plus completion per run, averaged over the 5 runs; the range is the lowest and highest run.

| Task | Control tokens | squeeze tokens | Saved | Tool bytes control -> squeeze | Turns control -> squeeze |
|---|---|---|---|---|---|
| `gotest-race` | 13010 (11539-15872) | 9310 (7181-12212) | 28% | 5.2 KB -> 2.9 KB | 4.4 -> 4.6 |
| `lint-log` | 67479 (64154-80366) | 6008 (5919-6134) | 91% | 31.4 KB -> 0.9 KB | 5.2 -> 5.0 |
| `pytest-failures` | 17215 (17042-17306) | 13052 (12831-13200) | 24% | 5.3 KB -> 4.9 KB | 6.0 -> 5.0 |
| `tflint-high` | 11587 (7169-15421) | 9671 (9537-9773) | 17% | 3.1 KB -> 1.9 KB | 5.6 -> 5.8 |
| all four | 109291 | 38041 | 65% | 46.1 KB -> 10.8 KB | 21.2 -> 20.4 |

What the `commands` of each run show:

- `lint-log` carries the total: the control agent reads the 400-line log once (32 KB) and then filters it with `grep -v ok`; the squeeze agent reads 8 lines, since the 399 `ok` lines fold into two notices around the `error:` line.
- `pytest-failures`: the control agent avoids pytest's output on its own with `pytest -q | tail -30` and `grep FAILED`, so the saving is one turn less rather than fewer bytes; both arms spend most tool bytes on `cat` of the source and the tests. squeeze's own output is 1.7 KB for 48 failures (4 distinct assertions), against 2.4 KB for the control's `tail -30`.
- `gotest-race` and `tflint-high`: the squeeze arm reads 2 KB less per run and takes the same number of turns; the model's variance between runs (7169 to 15421 tokens in the tflint control) is larger than the saving, so 5 runs settle the direction, not the size.
- In both arms, the agent runs the tool a second time with `>/dev/null; echo $?` to learn the exit code, which a pipe into squeeze hides. `--fail-on` gives squeeze the tool's verdict as its own exit code; the task prompts do not mention it.

A previous round at commit `fed4bb8`, before the two format fixes, measured `lint-log` at 15% and `pytest-failures` at -225%: every lint line carried a warning word and crowded the error out of the `--max-lines` budget, and 48 parametrized failures printed 48 tracebacks. The bench exists to find that kind of defect; read the `commands` before reading the percentages.

## Limitations

- Four tasks, each small: the number says whether the effect exists, not its size on real work.
- One agent loop and one prompt. A product agent has its own caps, retries and context handling; the saving there depends on where its caps sit.
- Token counts come from the API's `usage` object, so caching and pricing differences between models change the dollar figure but not the byte and turn counts.
