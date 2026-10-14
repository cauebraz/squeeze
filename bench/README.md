# Bench results

Model: `claude-opus-5-5`. Each task was solved 5 times without squeeze (control) and 5 times with squeeze. All 40 runs fixed the defect. Tokens are the mean per run.

| Task | Control | squeeze | Saved |
|---|---|---|---|
| `gotest-race` | 13010 | 9310 | 28% |
| `lint-log` | 67479 | 6008 | 91% |
| `pytest-failures` | 17215 | 13052 | 24% |
| `tflint-high` | 11587 | 9671 | 17% |
| total | 109291 | 38041 | 65% |

```
gotest-race      control  ############                                                       13010
                 squeeze  #########                                                           9310

lint-log         control  ################################################################   67479
                 squeeze  ######                                                              6008

pytest-failures  control  ################                                                   17215
                 squeeze  ############                                                       13052

tflint-high      control  ###########                                                        11587
                 squeeze  #########                                                           9671
```
