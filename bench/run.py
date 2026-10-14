#!/usr/bin/env python3
"""Measure what squeeze saves for a coding agent.

Each task is a small repository with a planted defect, a prompt and a
verifier. The same model solves it in two arms: control, where the agent
reads the tool's output as is, and squeeze, where the agent pipes the tool
through squeeze. The agent is a minimal tool-calling loop with one tool, a
shell in the task's working copy. Every run records the tokens the API
billed, the bytes the shell tool returned, the number of turns, and whether
the verifier passed.

Providers:
  anthropic   Anthropic Messages API, key in ANTHROPIC_API_KEY
  openai      OpenAI chat completions, key in OPENAI_API_KEY
  openrouter  OpenRouter chat completions, key in OPENROUTER_API_KEY
Without --provider, the first of these whose key is set is used. --base-url
points a provider at another endpoint with the same API, such as a local
OpenAI-compatible server; --api-key-env names another key variable.

Usage:
  bench/run.py --model <model id> [--provider anthropic|openai|openrouter]
      [--base-url URL] [--api-key-env NAME] [--runs 5]
      [--arm both|control|squeeze] [--task name ...] [--max-turns 30] [--jobs 1]
  bench/run.py --self-test            # setup, verify and solution of every task, no API calls
  bench/run.py --report bench/results/<file>.jsonl

Results go to bench/results/<timestamp>.jsonl, one line per run.
"""

import argparse
import concurrent.futures
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
PROVIDERS = {
    "anthropic": {"style": "anthropic", "url": "https://api.anthropic.com/v1/messages", "key": "ANTHROPIC_API_KEY"},
    "openai": {"style": "openai", "url": "https://api.openai.com/v1/chat/completions", "key": "OPENAI_API_KEY"},
    "openrouter": {"style": "openai", "url": "https://openrouter.ai/api/v1/chat/completions", "key": "OPENROUTER_API_KEY"},
}
ANTHROPIC_VERSION = "2023-06-01"
ANTHROPIC_MAX_TOKENS = 16000
TOOL_OUTPUT_CAP = 200_000
COMMAND_TIMEOUT = 300

SHELL_NAME = "shell"
SHELL_DESCRIPTION = "Run a shell command in the repository and return its exit code, stdout and stderr."
SHELL_SCHEMA = {
    "type": "object",
    "properties": {"command": {"type": "string", "description": "the command line, run with sh -c"}},
    "required": ["command"],
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


class Client:
    def __init__(self, provider, url, key, model):
        self.style = PROVIDERS[provider]["style"]
        self.provider, self.url, self.key, self.model = provider, url, key, model

    def start(self, system, prompt):
        if self.style == "anthropic":
            self.system = system
            return [{"role": "user", "content": prompt}]
        return [{"role": "system", "content": system}, {"role": "user", "content": prompt}]

    def body(self, messages):
        if self.style == "anthropic":
            tool = {"name": SHELL_NAME, "description": SHELL_DESCRIPTION, "input_schema": SHELL_SCHEMA}
            return {"model": self.model, "max_tokens": ANTHROPIC_MAX_TOKENS, "system": self.system,
                    "messages": messages, "tools": [tool]}
        tool = {"type": "function", "function": {"name": SHELL_NAME, "description": SHELL_DESCRIPTION, "parameters": SHELL_SCHEMA}}
        return {"model": self.model, "messages": messages, "tools": [tool], "tool_choice": "auto"}

    def headers(self):
        h = {"Content-Type": "application/json"}
        if self.style == "anthropic":
            h["x-api-key"] = self.key
            h["anthropic-version"] = ANTHROPIC_VERSION
        else:
            h["Authorization"] = "Bearer " + self.key
        if self.provider == "openrouter":
            h["HTTP-Referer"] = "https://github.com/cauebraz/squeeze"
            h["X-Title"] = "squeeze bench"
        return h

    def call(self, messages):
        req = urllib.request.Request(self.url, data=json.dumps(self.body(messages)).encode(), headers=self.headers())
        for attempt in range(5):
            try:
                with urllib.request.urlopen(req, timeout=300) as resp:
                    return self.parse(json.load(resp))
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

    def parse(self, resp):
        u = resp.get("usage") or {}
        if self.style == "anthropic":
            cached = u.get("cache_read_input_tokens") or 0
            usage = ((u.get("input_tokens") or 0) + (u.get("cache_creation_input_tokens") or 0) + cached,
                     u.get("output_tokens") or 0, cached)
            content = resp.get("content") or []
            calls = [(b["id"], (b.get("input") or {}).get("command", "")) for b in content if b.get("type") == "tool_use"]
            return usage, {"role": "assistant", "content": content}, calls
        usage = (u.get("prompt_tokens") or 0, u.get("completion_tokens") or 0,
                 (u.get("prompt_tokens_details") or {}).get("cached_tokens") or 0)
        msg = resp["choices"][0]["message"]
        calls = []
        for call in msg.get("tool_calls") or []:
            try:
                command = json.loads(call["function"].get("arguments") or "{}").get("command", "")
            except (json.JSONDecodeError, AttributeError):
                command = ""
            calls.append((call["id"], command))
        return usage, msg, calls

    def results(self, results):
        if self.style == "anthropic":
            return [{"role": "user", "content": [{"type": "tool_result", "tool_use_id": i, "content": t} for i, t in results]}]
        return [{"role": "tool", "tool_call_id": i, "content": t} for i, t in results]


def run_agent(client, task, work, env, max_turns):
    with open(os.path.join(work, "AGENTS.md")) as f:
        system = f.read()
    messages = client.start(system, task["prompt"] + "\n\nReply with the single word DONE when you are finished.")
    usage = {"prompt_tokens": 0, "completion_tokens": 0, "cached_tokens": 0}
    tool_bytes = 0
    turns = 0
    commands = []
    while turns < max_turns:
        turns += 1
        (prompt, completion, cached), msg, calls = client.call(messages)
        usage["prompt_tokens"] += prompt
        usage["completion_tokens"] += completion
        usage["cached_tokens"] += cached
        messages.append(msg)
        if not calls:
            break
        results = []
        for call_id, command in calls:
            if not isinstance(command, str):
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
            results.append((call_id, text))
        messages.extend(client.results(results))
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
    ap.add_argument("--model", help="model id as the provider names it, for example claude-opus-5-5 or anthropic/claude-opus-5.5")
    ap.add_argument("--provider", choices=sorted(PROVIDERS), help="default: the first provider whose key is set")
    ap.add_argument("--base-url", help="endpoint URL with the provider's API, for example a local OpenAI-compatible server")
    ap.add_argument("--api-key-env", help="environment variable that holds the key, instead of the provider's")
    ap.add_argument("--runs", type=int, default=5)
    ap.add_argument("--arm", choices=["both", "control", "squeeze"], default="both")
    ap.add_argument("--task", action="append", default=[])
    ap.add_argument("--max-turns", type=int, default=30)
    ap.add_argument("--jobs", type=int, default=1, help="runs in flight at once")
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
    provider = args.provider
    if not provider:
        provider = next((p for p in ("anthropic", "openai", "openrouter") if os.environ.get(PROVIDERS[p]["key"])), None)
        if not provider:
            sys.exit("set ANTHROPIC_API_KEY, OPENAI_API_KEY or OPENROUTER_API_KEY, or pass --provider and --api-key-env")
    key_env = args.api_key_env or PROVIDERS[provider]["key"]
    key = os.environ.get(key_env)
    if not key:
        sys.exit("set " + key_env)
    if not args.model:
        sys.exit("pass --model with a model id")
    client = Client(provider, args.base_url or PROVIDERS[provider]["url"], key, args.model)
    os.makedirs(RESULTS, exist_ok=True)
    out_path = os.path.join(RESULTS, dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ") + ".jsonl")
    arms = ["control", "squeeze"] if args.arm == "both" else [args.arm]
    print("writing %s (%s, %s)" % (out_path, provider, args.model))
    def one(i, name, task, arm):
        base, work, env = workdir(name, task, arm, squeeze_bin)
        start = time.time()
        usage, tool_bytes, turns, commands = run_agent(client, task, work, env, args.max_turns)
        passed = verify(task, work, env)
        if not args.keep:
            shutil.rmtree(base, ignore_errors=True)
        return {
            "task": name, "arm": arm, "run": i + 1, "provider": provider, "model": args.model, "passed": passed,
            "prompt_tokens": usage["prompt_tokens"], "completion_tokens": usage["completion_tokens"],
            "cached_tokens": usage["cached_tokens"], "tool_bytes": tool_bytes, "turns": turns,
            "seconds": round(time.time() - start, 1), "commands": commands,
        }

    cells = [(i, name, task, arm) for i in range(args.runs) for name, task in tasks.items() for arm in arms]
    with open(out_path, "w") as out, concurrent.futures.ThreadPoolExecutor(max_workers=max(1, args.jobs)) as pool:
        futures = [pool.submit(one, *c) for c in cells]
        for future in concurrent.futures.as_completed(futures):
            row = future.result()
            out.write(json.dumps(row) + "\n")
            out.flush()
            print("run %d %-18s %-8s %s  in=%d out=%d tool=%dKB turns=%d" % (
                row["run"], row["task"], row["arm"], "pass" if row["passed"] else "FAIL",
                row["prompt_tokens"], row["completion_tokens"], row["tool_bytes"] // 1024, row["turns"]))


if __name__ == "__main__":
    main()
