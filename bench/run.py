#!/usr/bin/env python3
"""Measure what squeeze saves for a coding agent.

Each task is a small repository with a planted defect, a prompt and a
verifier. The same model solves it in two arms: control, where the agent
reads the tool's output as is, and squeeze, where the agent pipes the tool
through squeeze. The agent is a minimal tool-calling loop over the
OpenRouter chat completions API with one tool, a shell in the task's
working copy. Every run records the tokens the API billed, the bytes the
shell tool returned, the number of turns, and whether the verifier passed.

Usage:
  OPENROUTER_API_KEY=... bench/run.py --model <openrouter model id> [--runs 5]
      [--arm both|control|squeeze] [--task name ...] [--max-turns 30]
  bench/run.py --self-test            # setup, verify and solution of every task, no API calls
  bench/run.py --report bench/results/<file>.jsonl

Results go to bench/results/<timestamp>.jsonl, one line per run.
"""

import argparse
import datetime as dt
import json
import os
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(ROOT)
TASKS = os.path.join(ROOT, "tasks")
RESULTS = os.path.join(ROOT, "results")
API = "https://openrouter.ai/api/v1/chat/completions"
TOOL_OUTPUT_CAP = 200_000
COMMAND_TIMEOUT = 300

SHELL_TOOL = {
    "type": "function",
    "function": {
        "name": "shell",
        "description": "Run a shell command in the repository and return its exit code, stdout and stderr.",
        "parameters": {
            "type": "object",
            "properties": {"command": {"type": "string", "description": "the command line, run with sh -c"}},
            "required": ["command"],
        },
    },
}

AGENTS_HEADER = """# Agent guidelines

You are fixing a small repository. Work only inside the current directory.
Use the shell tool to read files, run commands and edit files (for example with
`cat`, `sed -i` or a heredoc). Keep each command's output small: list and read
only what you need.

"""

SQUEEZE_BLOCK = """- squeeze is installed. Pipe tool output through it as the lines below say and
  read squeeze's output instead of the tool's. When squeeze reports cut lines,
  rerun with the flag it names.
"""


def load_tasks(names):
    tasks = {}
    for name in sorted(os.listdir(TASKS)):
        path = os.path.join(TASKS, name, "task.json")
        if not os.path.isfile(path):
            continue
        if names and name not in names:
            continue
        with open(path) as f:
            tasks[name] = json.load(f)
    if names:
        missing = [n for n in names if n not in tasks]
        if missing:
            sys.exit("unknown tasks: " + ", ".join(missing))
    return tasks


def sh(command, cwd, env=None, timeout=COMMAND_TIMEOUT):
    try:
        p = subprocess.run(
            ["sh", "-c", command], cwd=cwd, env=env, capture_output=True, timeout=timeout
        )
        return p.returncode, p.stdout.decode(errors="replace"), p.stderr.decode(errors="replace")
    except subprocess.TimeoutExpired:
        return 124, "", "command timed out after %ds" % timeout


def workdir(name, task, arm, squeeze_bin):
    base = tempfile.mkdtemp(prefix="squeeze-bench-%s-%s-" % (name, arm))
    work = os.path.join(base, "repo")
    shutil.copytree(os.path.join(TASKS, name, "repo"), work, symlinks=True)
    env = dict(os.environ)
    if arm == "squeeze":
        bindir = os.path.join(base, "bin")
        os.makedirs(bindir)
        shutil.copy(squeeze_bin, os.path.join(bindir, "squeeze"))
        env["PATH"] = bindir + os.pathsep + env["PATH"]
    if task.get("setup"):
        code, out, err = sh(task["setup"], work, env)
        if code != 0:
            sys.exit("setup of %s failed (%d):\n%s%s" % (name, code, out, err))
    with open(os.path.join(work, "AGENTS.md"), "w") as f:
        f.write(AGENTS_HEADER)
        if arm == "squeeze":
            f.write(SQUEEZE_BLOCK)
        f.write(task[arm] + "\n")
    return base, work, env


def build_squeeze():
    out = os.path.join(tempfile.gettempdir(), "squeeze-bench-bin")
    p = subprocess.run(["go", "build", "-o", out, "./cmd/squeeze"], cwd=REPO, capture_output=True)
    if p.returncode != 0:
        sys.exit("go build failed:\n" + p.stderr.decode())
    return out


def call_api(key, model, messages, tools):
    body = json.dumps({"model": model, "messages": messages, "tools": tools, "tool_choice": "auto"}).encode()
    req = urllib.request.Request(
        API,
        data=body,
        headers={
            "Authorization": "Bearer " + key,
            "Content-Type": "application/json",
            "HTTP-Referer": "https://github.com/cauebraz/squeeze",
            "X-Title": "squeeze bench",
        },
    )
    for attempt in range(5):
        try:
            with urllib.request.urlopen(req, timeout=300) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as e:
            text = e.read().decode(errors="replace")
            if e.code in (429, 500, 502, 503, 529) and attempt < 4:
                time.sleep(2 ** attempt * 2)
                continue
            sys.exit("API error %d: %s" % (e.code, text[:500]))
        except (urllib.error.URLError, TimeoutError):
            if attempt < 4:
                time.sleep(2 ** attempt * 2)
                continue
            raise


def run_agent(key, model, task, work, env, max_turns):
    with open(os.path.join(work, "AGENTS.md")) as f:
        system = f.read()
    messages = [
        {"role": "system", "content": system},
        {"role": "user", "content": task["prompt"] + "\n\nReply with the single word DONE when you are finished."},
    ]
    usage = {"prompt_tokens": 0, "completion_tokens": 0, "cached_tokens": 0}
    tool_bytes = 0
    turns = 0
    commands = []
    while turns < max_turns:
        turns += 1
        resp = call_api(key, model, messages, [SHELL_TOOL])
        u = resp.get("usage") or {}
        usage["prompt_tokens"] += u.get("prompt_tokens", 0)
        usage["completion_tokens"] += u.get("completion_tokens", 0)
        usage["cached_tokens"] += (u.get("prompt_tokens_details") or {}).get("cached_tokens", 0)
        msg = resp["choices"][0]["message"]
        messages.append(msg)
        calls = msg.get("tool_calls") or []
        if not calls:
            break
        for call in calls:
            try:
                args = json.loads(call["function"].get("arguments") or "{}")
                command = args.get("command", "")
            except json.JSONDecodeError:
                command = ""
            commands.append(command)
            code, out, err = sh(command, work, env)
            text = "exit code: %d\n" % code
            if out:
                text += "stdout:\n" + out
            if err:
                text += "stderr:\n" + err
            if len(text) > TOOL_OUTPUT_CAP:
                text = text[:TOOL_OUTPUT_CAP] + "\n[output cut at %d bytes]" % TOOL_OUTPUT_CAP
            tool_bytes += len(text.encode())
            messages.append({"role": "tool", "tool_call_id": call["id"], "content": text})
    return usage, tool_bytes, turns, commands


def verify(task, work, env):
    code, _, _ = sh(task["verify"], work, env)
    return code == 0


def self_test(tasks, squeeze_bin):
    ok = True
    for name, task in tasks.items():
        base, work, env = workdir(name, task, "squeeze", squeeze_bin)
        before = verify(task, work, env)
        code, out, err = sh(task["solution"], work, env)
        after = verify(task, work, env)
        shutil.rmtree(base)
        status = "ok" if (not before and after and code == 0) else "BAD"
        if status == "BAD":
            ok = False
        print("%-18s verify before fix: %s, solution exit %d, verify after fix: %s -> %s" % (name, before, code, after, status))
        if err.strip():
            print("  solution stderr: " + err.strip()[:300])
    return ok


def report(path):
    rows = [json.loads(l) for l in open(path) if l.strip()]
    if not rows:
        print("no runs")
        return
    by = {}
    for r in rows:
        by.setdefault((r["task"], r["arm"]), []).append(r)
    print("%-18s %-8s %4s %6s %10s %10s %10s %6s" % ("task", "arm", "runs", "pass", "in_tok", "out_tok", "tool_KB", "turns"))
    for (task, arm) in sorted(by):
        rs = by[(task, arm)]
        n = len(rs)
        passed = sum(1 for r in rs if r["passed"])
        print("%-18s %-8s %4d %3d/%-2d %10.0f %10.0f %10.1f %6.1f" % (
            task, arm, n, passed, n,
            statistics.mean(r["prompt_tokens"] for r in rs),
            statistics.mean(r["completion_tokens"] for r in rs),
            statistics.mean(r["tool_bytes"] for r in rs) / 1024,
            statistics.mean(r["turns"] for r in rs),
        ))
    tasks = sorted({t for t, _ in by})
    print()
    print("%-18s %12s %12s %8s" % ("task", "control_tok", "squeeze_tok", "saved"))
    for task in tasks:
        c = by.get((task, "control"))
        s = by.get((task, "squeeze"))
        if not c or not s:
            continue
        ct = statistics.mean(r["prompt_tokens"] + r["completion_tokens"] for r in c)
        st = statistics.mean(r["prompt_tokens"] + r["completion_tokens"] for r in s)
        print("%-18s %12.0f %12.0f %7.0f%%" % (task, ct, st, (1 - st / ct) * 100 if ct else 0))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--model", help="OpenRouter model id, for example anthropic/claude-sonnet-4.5")
    ap.add_argument("--runs", type=int, default=5)
    ap.add_argument("--arm", choices=["both", "control", "squeeze"], default="both")
    ap.add_argument("--task", action="append", default=[])
    ap.add_argument("--max-turns", type=int, default=30)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--report", metavar="FILE")
    ap.add_argument("--keep", action="store_true", help="keep the working copies")
    args = ap.parse_args()

    if args.report:
        report(args.report)
        return
    tasks = load_tasks(args.task)
    squeeze_bin = build_squeeze()
    if args.self_test:
        sys.exit(0 if self_test(tasks, squeeze_bin) else 1)
    key = os.environ.get("OPENROUTER_API_KEY")
    if not key:
        sys.exit("set OPENROUTER_API_KEY")
    if not args.model:
        sys.exit("pass --model with an OpenRouter model id")
    os.makedirs(RESULTS, exist_ok=True)
    out_path = os.path.join(RESULTS, dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + ".jsonl")
    arms = ["control", "squeeze"] if args.arm == "both" else [args.arm]
    print("writing " + out_path)
    with open(out_path, "w") as out:
        for i in range(args.runs):
            for name, task in tasks.items():
                for arm in arms:
                    base, work, env = workdir(name, task, arm, squeeze_bin)
                    start = time.time()
                    usage, tool_bytes, turns, commands = run_agent(key, args.model, task, work, env, args.max_turns)
                    passed = verify(task, work, env)
                    row = {
                        "task": name, "arm": arm, "run": i + 1, "model": args.model, "passed": passed,
                        "prompt_tokens": usage["prompt_tokens"], "completion_tokens": usage["completion_tokens"],
                        "cached_tokens": usage["cached_tokens"], "tool_bytes": tool_bytes, "turns": turns,
                        "seconds": round(time.time() - start, 1), "commands": commands,
                    }
                    out.write(json.dumps(row) + "\n")
                    out.flush()
                    print("run %d %-18s %-8s %s  in=%d out=%d tool=%dKB turns=%d" % (
                        i + 1, name, arm, "pass" if passed else "FAIL",
                        usage["prompt_tokens"], usage["completion_tokens"], tool_bytes // 1024, turns))
                    if args.keep:
                        print("  kept " + work)
                    else:
                        shutil.rmtree(base)
    print()
    report(out_path)


if __name__ == "__main__":
    main()
