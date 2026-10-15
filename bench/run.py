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
  bench/run.py --chart bench/results/<file>.jsonl [--chart-out bench/results.svg]

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

MACOS_NOTE = """The shell runs on macOS: `sed -i` takes a backup suffix argument, so write
`sed -i '' 's/a/b/' file`, or use `perl -pi -e` or a heredoc.

"""

SQUEEZE_BLOCK = """- squeeze is installed. Pipe tool output through it as the lines below say and
  read squeeze's output instead of the tool's. When squeeze reports cut lines,
  rerun with the flag it names only if what you need is not in the output.
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
        if sys.platform == "darwin":
            f.write(MACOS_NOTE)
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


def change(saved):
    pct = round(-saved * 100)
    return "0%" if pct == 0 else "%+d%%" % pct


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


CHART_LIGHT = {"surface": "#fcfcfb", "ink": "#0b0b0b", "ink2": "#52514e", "muted": "#898781", "grid": "#e1e0d9",
               "base": "#c3c2b7", "squeeze": "#2a78d6", "control": "#eb6834", "border": "rgba(11,11,11,0.10)"}
CHART_DARK = {"surface": "#1a1a19", "ink": "#ffffff", "ink2": "#c3c2b7", "muted": "#898781", "grid": "#2c2c2a",
              "base": "#383835", "squeeze": "#3987e5", "control": "#d95926", "border": "rgba(255,255,255,0.10)"}
CHART_STYLE = """text { font-family: system-ui, -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; }
.kicker { font-size: 13px; font-weight: 600; letter-spacing: 0.04em; }
.title { font-size: 24px; font-weight: 650; }
.sub, .herol { font-size: 13px; }
.hero { font-size: 44px; font-weight: 700; }
.task { font-size: 14px; font-weight: 600; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.val { font-size: 12px; font-variant-numeric: tabular-nums; }
.saved { font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
.savedl, .tick, .foot { font-size: 11px; }
.tick { font-variant-numeric: tabular-nums; }
.leg { font-size: 12px; }
.grid, .base { stroke-width: 1; }
.wh { stroke-width: 1.5; opacity: 0.55; }"""


def chart_colors(c):
    return (".bg{fill:%(surface)s;stroke:%(border)s} text{fill:%(ink)s} .kicker{fill:%(squeeze)s} "
            ".sub,.herol,.val,.leg{fill:%(ink2)s} .savedl,.tick,.foot{fill:%(muted)s} .grid{stroke:%(grid)s} "
            ".base{stroke:%(base)s} .squeeze{fill:%(squeeze)s} .control{fill:%(control)s} .wh{stroke:%(ink)s}") % c


def chart_step(top):
    step = 1
    while True:
        for m in (1, 2, 5):
            if top / (step * m) <= 5:
                return step * m
        step *= 10


def chart(path, out_path):
    rows = [json.loads(l) for l in open(path) if l.strip()]
    by = {}
    for r in rows:
        by.setdefault((r["task"], r["arm"]), []).append(r["prompt_tokens"] + r["completion_tokens"])
    stats = []
    for task in sorted({t for t, _ in by}):
        c, s = by.get((task, "control")), by.get((task, "squeeze"))
        if c and s:
            cm, sm = statistics.mean(c), statistics.mean(s)
            stats.append({"task": task, "control": (cm, c), "squeeze": (sm, s), "saved": 1 - sm / cm})
    if not stats:
        sys.exit("%s has no task with both arms" % path)
    stats.sort(key=lambda d: -d["saved"])
    total_c = sum(d["control"][0] for d in stats)
    total_s = sum(d["squeeze"][0] for d in stats)
    runs = max(len(v) for v in by.values())
    passed = sum(1 for r in rows if r["passed"])
    model = rows[0]["model"].split("/")[-1]

    width, left, x0, plot_w = 880, 32, 182, 520
    right = width - 32
    top, bar_h, pair_gap, group_gap = 168, 18, 2, 30
    group_h = 2 * bar_h + pair_gap
    plot_bottom = top + len(stats) * group_h + (len(stats) - 1) * group_gap
    height = plot_bottom + 92
    hi_all = max(max(v) for v in by.values())
    step = chart_step(hi_all)
    xmax = hi_all * 1.02

    def sx(v):
        return x0 + plot_w * v / xmax

    def num(v):
        return "{:,}".format(round(v))

    def esc(s):
        return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace('"', "&quot;")

    o = ['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-labelledby="t d">'
         % (width, height, width, height),
         '<title id="t">squeeze bench: tokens per task with and without squeeze</title>',
         '<desc id="d">%s</desc>' % esc("; ".join("%s: %s tokens without squeeze, %s with squeeze"
                                                   % (d["task"], num(d["control"][0]), num(d["squeeze"][0])) for d in stats)),
         "<style>", CHART_STYLE, chart_colors(CHART_LIGHT),
         "@media (prefers-color-scheme: dark) { %s }" % chart_colors(CHART_DARK), "</style>",
         '<rect class="bg" x="0.5" y="0.5" width="%d" height="%d" rx="12"/>' % (width - 1, height - 1),
         '<text class="kicker" x="%d" y="40">SQUEEZE BENCH</text>' % left,
         '<text class="title" x="%d" y="70">Tokens an agent spends to fix a defect</text>' % left,
         '<text class="sub" x="%d" y="94">%s - %d runs per task and arm - %d/%d runs fixed the defect</text>'
         % (left, esc(model), runs, passed, len(rows)),
         '<text class="hero" x="%d" y="76" text-anchor="end">%.0f%%</text>' % (right, (1 - total_s / total_c) * 100),
         '<text class="herol" x="%d" y="98" text-anchor="end">fewer tokens: %s -&gt; %s</text>' % (right, num(total_c), num(total_s))]
    ly = 132
    o += ['<rect class="squeeze" x="%d" y="%d" width="12" height="12" rx="3"/>' % (x0, ly - 10),
          '<text class="leg" x="%d" y="%d">with squeeze</text>' % (x0 + 18, ly),
          '<rect class="control" x="%d" y="%d" width="12" height="12" rx="3"/>' % (x0 + 122, ly - 10),
          '<text class="leg" x="%d" y="%d">without squeeze</text>' % (x0 + 140, ly),
          '<line class="wh" x1="%d" x2="%d" y1="%d" y2="%d"/>' % (x0 + 266, x0 + 286, ly - 4, ly - 4),
          '<text class="leg" x="%d" y="%d">min-max of %d runs</text>' % (x0 + 292, ly, runs),
          '<text class="savedl" x="%d" y="%d" text-anchor="end">tokens</text>' % (right, ly)]
    for v in range(0, int(xmax) + 1, step):
        label = "%gk" % (v / 1000) if step >= 1000 else str(v)
        o += ['<line class="grid" x1="%.1f" x2="%.1f" y1="%d" y2="%d"/>' % (sx(v), sx(v), top - 8, plot_bottom + 8),
              '<text class="tick" x="%.1f" y="%d" text-anchor="middle">%s</text>' % (sx(v), plot_bottom + 26, label)]
    o.append('<line class="base" x1="%d" x2="%d" y1="%d" y2="%d"/>' % (x0, x0, top - 8, plot_bottom + 8))
    y = top
    for d in stats:
        o.append('<text class="task" x="%d" y="%.1f">%s</text>' % (left, y + group_h / 2 + 5, esc(d["task"])))
        for i, (arm, label) in enumerate((("squeeze", "with squeeze"), ("control", "without squeeze"))):
            mean, vals = d[arm]
            by_, w = y + i * (bar_h + pair_gap), sx(mean) - x0
            r = min(4, w / 2, bar_h / 2)
            o.append('<path class="%s" d="M%d,%d h%.1f a%.1f,%.1f 0 0 1 %.1f,%.1f v%.1f a%.1f,%.1f 0 0 1 -%.1f,%.1f h-%.1f z">'
                     '<title>%s %s: mean %s, min %s, max %s tokens</title></path>'
                     % (arm, x0, by_, w - r, r, r, r, r, bar_h - 2 * r, r, r, r, r, w - r,
                        esc(d["task"]), label, num(mean), num(min(vals)), num(max(vals))))
            cy, lo, hi = by_ + bar_h / 2, sx(min(vals)), sx(max(vals))
            if hi - lo >= 2:
                o += ['<line class="wh" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>' % (lo, hi, cy, cy),
                      '<line class="wh" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>' % (lo, lo, cy - 4, cy + 4),
                      '<line class="wh" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>' % (hi, hi, cy - 4, cy + 4)]
            o.append('<text class="val" x="%.1f" y="%.1f">%s</text>' % (max(sx(mean), hi) + 8, cy + 4, num(mean)))
        o.append('<text class="saved" x="%d" y="%.1f" text-anchor="end">%s</text>' % (right, y + group_h / 2 + 6, change(d["saved"])))
        y += group_h + group_gap
    o += ['<text class="foot" x="%d" y="%d">Mean prompt + completion tokens per run, from %s.</text>'
          % (left, height - 22, esc(os.path.basename(path))), "</svg>"]
    with open(out_path, "w") as f:
        f.write("\n".join(o) + "\n")
    print("wrote %s" % out_path)


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
    ap.add_argument("--chart", metavar="FILE", help="write the token chart of a results file to --chart-out")
    ap.add_argument("--chart-out", default=os.path.join(ROOT, "results.svg"), metavar="SVG")
    ap.add_argument("--keep", action="store_true", help="keep the working copies")
    args = ap.parse_args()

    if args.report:
        report(args.report)
        return
    if args.chart:
        chart(args.chart, args.chart_out)
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
