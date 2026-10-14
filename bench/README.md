# Bench results

Model: `claude-opus-5-5`. Each task was solved 5 times without squeeze (control) and 5 times with squeeze. All 40 runs fixed the defect. Tokens are the mean per run.

![Tokens an agent spends per task with and without squeeze](results.svg)

`just chart bench/results/<file>.jsonl` redraws `results.svg` from a run; the lines on each bar span the fewest and most tokens of the 5 runs.

| Task | Control | squeeze | Saved |
|---|---|---|---|
| `gotest-race` | 13010 | 9310 | 28% |
| `lint-log` | 67479 | 6008 | 91% |
| `pytest-failures` | 17215 | 13052 | 24% |
| `tflint-high` | 11587 | 9671 | 17% |
| total | 109291 | 38041 | 65% |
