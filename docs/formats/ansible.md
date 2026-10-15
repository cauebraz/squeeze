# ansible

Input: the output of `ansible-playbook` with the default stdout callback, at any verbosity, or the document of the `ansible.posix.json` stdout callback (https://docs.ansible.com/ansible/latest/collections/ansible/posix/json_callback.html). The text is detected when it holds a `PLAY [<name>] ***` banner and a `TASK [...]`, `RUNNING HANDLER [...]` or `PLAY RECAP` banner, or `skipping: no hosts matched`; color codes are removed first. The JSON is detected from the first line that starts with `{` when it holds the keys `custom_stats`, `global_custom_stats` and `plays`, which the callback writes first because it sorts its keys; text before it, such as a `[WARNING]` line from `2>&1`, is read for warnings and errors.

```
ansible-playbook -i inventory.ini site.yml 2>&1 | squeeze
ANSIBLE_STDOUT_CALLBACK=ansible.posix.json ansible-playbook site.yml | squeeze ansible --all
```

The default output prints a banner for every play and task, a line per host and loop item, and for every failure the module result as JSON, the `[ERROR]` message again and, from ansible-core 2.19 on, the playbook lines around the task. The JSON callback adds the full result of every task, with `invocation.module_args` when `ANSIBLE_INJECT_INVOCATION` is set (ansible-core 2.21) or the module returns it (earlier versions). squeeze keeps the recap, the failed and unreachable tasks with their message and output, and counts the rest. On the fixtures, captured from ansible-core 2.21.4 and ansible.posix 2.2.2, the text is 588 of 4,856 bytes for the default output, 592 of 34,910 bytes at `-vvv` and 577 of 34,212 bytes for the JSON callback.

## Output

```
$ squeeze < internal/formats/ansible/testdata/failed/input.txt
ansible-playbook: 2 plays, 2 hosts, 1 failed, 1 unreachable
  localhost: ok=10 changed=7 unreachable=0 failed=1 skipped=1 rescued=1 ignored=3
  web1: ok=0 changed=0 unreachable=1 failed=0 skipped=0 rescued=0 ignored=0
unreachable web1: Check the app directory /home/dev/shop/site.yml:31:7
  msg: Task failed: Failed to connect to the host via ssh: ssh: connect to host 10.255.255.1 port 22: Connection refused
failed localhost: Assert the release is current /home/dev/shop/site.yml:74:7
  msg: release 43 is not deployed
... 5 changed tasks, 3 ignored failures, 1 rescued failure (--all)
```

- The header names the version when the output prints it (`ansible-playbook [core 2.21.4]` at `-vv` and above), the plays, the hosts in the recap, and the hosts with `failed` or `unreachable` above 0. Without a `PLAY RECAP` the header says `no PLAY RECAP` and counts hosts from the failures it read.
- Each recap line prints as Ansible writes it: `ok`, `changed`, `unreachable`, `failed`, `skipped`, `rescued`, `ignored`. Hosts with failures or unreachable always print; other hosts are cut at `--max-hosts`, announced as `... N hosts (--max-hosts 0)`.
- `[ERROR]`, `ERROR!`, `[WARNING]` and `[DEPRECATION WARNING]` lines print as `error:` and `warning:`, except an `[ERROR]` that Ansible prints for a task result that follows it, which is the same message.
- Each failed or unreachable task result prints as `<status> <host>: <task> (item=<label>) <path>`, with one line per failed loop item. The path comes from `task path:` (`-vv`), the `Origin:` line of the error (ansible-core 2.19 on) or the callback's `task.path`.
- Under it print the result's `msg`, `rc`, `stderr`, `stdout`, `module_stderr`, `module_stdout` and `censored`, each when set; a value with several lines is cut to its last `--max-lines` lines, announced as `... N lines cut (--max-lines 0)`. The other keys of the result, `invocation` and its `module_args` among them, are never printed or kept, and neither are the playbook lines after `Origin:`.
- A failure followed by `...ignoring` is `ignored`. Ansible counts a failure in a block with a `rescue` section as `rescued` and any other one as `failed` (`lib/ansible/plugins/strategy/__init__.py` in ansible-core 2.21.4), and a host that fails runs no further task, so per host the last `failed` failures of the recap stay `failed` and the earlier ones are `rescued` when the recap counts any. Without a recap every failure stays `failed`.
- Failed and unreachable results print first, cut at `--max-tasks`. `--all` adds every changed task result once per host, and the ignored and rescued failures with their output; without it they are counted in `... N changed tasks, N ignored failures, N rescued failures (--all)`. The recap's `changed` also counts ignored failures that reported a change, so it can be higher than the changed tasks listed.
- Names, messages and output lines lose control characters and escape sequences; tabs print as four spaces.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--all` | off | also list changed tasks and the failures the run ignored or rescued |
| `--max-hosts` | `20` | recap lines of hosts without failures, `0` for all |
| `--max-lines` | `20` | last lines of each msg, stderr or stdout, `0` for all |
| `--max-tasks` | `20` | task results listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `changed` | a task result reported a change, for example in a `--check` run |
| `error` | a host failed or was unreachable, or the output has an `[ERROR]` or `ERROR!` line of its own |
| `fail` | a host failed or was unreachable: `ansible-playbook` exits with 2 or 4 |
| `unreachable` | a host was unreachable |
| `warning` | the output has a `[WARNING]` or `[DEPRECATION WARNING]` line |

## JSON data

`data` holds `tool` (`ansible-playbook`), `tool_version` (empty when the output does not print it), `input` (`text` or `json`), `plays`, `recap` (whether a recap was read), `hosts` (the printed recap lines: `host`, `ok`, `changed`, `unreachable`, `failed`, `skipped`, `rescued`, `ignored`), `hosts_omitted`, `failed` and `unreachable` (hosts), `changed`, `ignored`, `rescued` and `recovered` (task results), `tasks`, `tasks_omitted`, `errors` and `warnings`. Each task has `status`, `host`, `play`, `task`, `path`, `item` when it is a loop item, and the `msg`, `rc`, `stderr`, `stdout`, `module_stderr`, `module_stdout` and `censored` that are set, each whole.

## Limitations

- The JSON callback does not record `ignore_errors` in a result. A failure the host ran past is `ignored` when the recap counts only ignored failures for that host, `rescued` when it counts only rescued ones, and `recovered` when it counts both.
- ansible-core 2.19 and later do not pass `failed` to callbacks for loop items (`lib/ansible/_internal/_task.py`, the `_failed` field). In the JSON callback an item counts as failed when it has `failed: true`, a true `failed_when_result`, a non-zero `rc` or an `exception`.
- The `ansible.posix.jsonl` callback, `ansible-playbook --syntax-check` and ad hoc `ansible` commands are not read. `ansible.posix.json` is not part of ansible-core: install the `ansible.posix` collection and set `ANSIBLE_STDOUT_CALLBACK=ansible.posix.json`.
- With the `free` or `host_pinned` strategy, results of different tasks interleave; a result is attributed to the task of the last banner, and Ansible prints that banner again before each switch.
- A `msg` that is a list or an object prints as compact JSON.
