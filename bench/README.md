# Bench results

Model: `claude-opus-5-5`. Each task was solved 5 times without squeeze (control) and 5 times with squeeze. All 80 runs fixed the defect. Tokens are the mean per run.

![Tokens an agent spends per task with and without squeeze](results.svg)

`just chart bench/results/<file>.jsonl` redraws `results.svg` from a run; the lines on each bar span the fewest and most tokens of the 5 runs.

## Every task

The result files named below are not in the repository: `.gitignore` excludes `bench/results/`. `bench/run.py --report bench/results/<file>.jsonl` regenerates these tables from a run. Tokens are prompt plus completion, mean per run; `saved` is `1 - squeeze / control`.

The chart, `bench/results/20261015T110129Z.jsonl`, 5 runs per arm. All tasks together save 47 percent; without `lint-log`, 28 percent; the median task saves 27 percent. Across all 13 tasks of both tables the median task, `precommit-hooks`, saves 23 percent.

| Task | Format | Control | Squeeze | Saved |
|---|---|---|---|---|
| `lint-log` | `generic` | 48,264 | 5,127 | 89% |
| `npmaudit-fix` | `npmaudit` | 14,027 | 6,464 | 54% |
| `tflint-high` | `tflint` | 13,101 | 8,721 | 33% |
| `junit-report` | `junit` | 25,722 | 17,613 | 32% |
| `precommit-hooks` | `precommit` | 21,554 | 16,667 | 23% |
| `gotest-race` | `gotest` | 10,169 | 7,927 | 22% |
| `jest-failures` | `jest` | 12,365 | 10,793 | 13% |
| `pytest-failures` | `pytest` | 11,102 | 9,700 | 13% |

The other tasks, `bench/results/20261015T113457Z.jsonl`, 10 runs per arm:

| Task | Format | Control | Squeeze | Saved |
|---|---|---|---|---|
| `ghactions-log` | `ghactions` | 175,153 | 14,921 | 91% |
| `tfstate-attributes` | `tfstate` | 14,396 | 9,742 | 32% |
| `trivy-vulns` | `trivy` | 31,962 | 26,218 | 18% |
| `tfvalidate-errors` | `tfvalidate` | 10,650 | 11,626 | -9% |
| `checkov-findings` | `checkov` | 12,184 | 13,374 | -10% |

## Where squeeze loses

- `checkov-findings`: the control agent filtered Checkov's text output with `grep` and read 4.5 KB per run; the squeeze arm read 5.3 KB of squeezed JSON.
- `tfvalidate-errors`: the squeeze arm read less tool output (3.1 KB against 3.6 KB) in the same 6 turns, but sent 1,012 more prompt tokens per run. The 456 bytes of squeeze lines in the system prompt are resent on every turn, about 680 tokens over 6 turns at 4 bytes per token, so on a small output they can cost more than the cut saves.
- A single run varies: `pytest-failures` went from -225 percent in `20261014T013245Z.jsonl` to 24 percent in the next run with the same tasks. Read a task's result across runs, not from one.

## Formats removed

`tsc`, `cargo`, `golangci` and `ansible` were removed in `4b3d04f` after `bench/results/20261015T104346Z.jsonl` (10 runs per arm) measured no saving worth a format:

| Task | Control | Squeeze | Saved |
|---|---|---|---|
| `ansible-failed` | 15,794 | 14,804 | 6% |
| `golangci-issues` | 16,379 | 16,033 | 2% |
| `tsc-errors` | 11,371 | 11,286 | 1% |
| `cargo-errors` | 15,021 | 16,899 | -13% |
