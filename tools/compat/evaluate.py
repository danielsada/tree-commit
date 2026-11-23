"""Run tree-commit and pre-commit on the same repositories and compare per-hook results.

Usage: python3 evaluate.py results.json repos.txt

Environment: TC_EVAL_DIR (work directory, default /var/tmp/tc-eval), TC_BINARY
(tree-commit binary, default $TC_EVAL_DIR/tree-commit), TC_EVAL_JOBS (repos in
parallel, default 1: keep 1 for meaningful timings). uv must be on PATH.

CAUTION: this runs the hooks of the listed repositories, i.e. third-party code.
Run it in a disposable environment (container, VM or WSL).
"""

import json
import os
import re
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor

BASE = os.environ.get("TC_EVAL_DIR", "/var/tmp/tc-eval")
OUT = sys.argv[1]
TC = os.environ.get("TC_BINARY", f"{BASE}/tree-commit")
ENV = dict(
    os.environ,
    TREE_COMMIT_HOME=f"{BASE}/tc-home",
    PRE_COMMIT_HOME=f"{BASE}/pc-home",
    UV_CACHE_DIR=f"{BASE}/uv-cache",
    NO_COLOR="1",
    HOME=os.environ.get("HOME", "/tmp"),
)
STATUS = re.compile(r"^(?P<name>.+?)\.{3,}(?:\([^)]*\))?(?P<status>Passed|Failed|Skipped)$")


def sh(cmd, cwd, timeout=1800):
    start = time.time()
    try:
        p = subprocess.run(cmd, cwd=cwd, env=ENV, capture_output=True, text=True, timeout=timeout)
        return p.returncode, p.stdout + p.stderr, time.time() - start
    except subprocess.TimeoutExpired as e:
        out = (e.stdout or b"").decode() if isinstance(e.stdout, bytes) else (e.stdout or "")
        return "timeout", out, time.time() - start


def parse(out, strip_scope):
    res = {}
    for line in out.splitlines():
        m = STATUS.match(line.strip())
        if m:
            name = m["name"].strip()
            if strip_scope:
                name = re.sub(r" \([^()]*/\)$", "", name)
            res.setdefault(name, []).append(m["status"])
    return res


def reset(d):
    subprocess.run(["git", "checkout", "-q", "--", "."], cwd=d, env=ENV)
    subprocess.run(["git", "clean", "-fdqx", "-e", ".venv"], cwd=d, env=ENV)


def evaluate(name):
    d = f"{BASE}/repos/{name.replace('/', '__')}"
    r = {"repo": name}
    if not os.path.isdir(d):
        code, out, _ = sh(
            ["git", "clone", "-q", "--depth", "1", f"https://github.com/{name}.git", d], BASE, 600
        )
        if code != 0:
            r["error"] = "clone failed: " + out[-500:]
            return r
    r["configs"] = subprocess.run(
        ["git", "ls-files", "*.pre-commit-config.yaml", "*.tree-commit-config.yaml"],
        cwd=d,
        capture_output=True,
        text=True,
    ).stdout.split()
    r["files"] = len(
        subprocess.run(
            ["git", "ls-files"], cwd=d, capture_output=True, text=True
        ).stdout.splitlines()
    )
    reset(d)
    code, out, dur = sh([TC, "run", "--all-files", "--color", "never"], d)
    r["tc"] = {"code": code, "secs": round(dur), "hooks": parse(out, True), "out": out[-20000:]}
    r["tc_modified"] = subprocess.run(
        ["git", "status", "--porcelain"], cwd=d, capture_output=True, text=True
    ).stdout.count("\n")
    reset(d)
    code, out, dur = sh(["uvx", "pre-commit@4.3.0", "run", "--all-files", "--color", "never"], d)
    r["pc"] = {"code": code, "secs": round(dur), "hooks": parse(out, False), "out": out[-20000:]}
    r["pc_modified"] = subprocess.run(
        ["git", "status", "--porcelain"], cwd=d, capture_output=True, text=True
    ).stdout.count("\n")
    reset(d)
    return r


repos = [line.strip() for line in open(sys.argv[2]) if line.strip()]
os.makedirs(f"{BASE}/repos", exist_ok=True)
with ThreadPoolExecutor(int(os.environ.get("TC_EVAL_JOBS", "1"))) as pool:
    results = list(pool.map(evaluate, repos))
json.dump(results, open(OUT, "w"), indent=1)
for r in results:
    if "error" in r:
        print(r["repo"], r["error"])
        continue
    tc, pc = r["tc"]["hooks"], r["pc"]["hooks"]
    same = sum(1 for k in pc if tc.get(k) == pc[k])
    print(
        f"{r['repo']:42} files={r['files']:<5}",
        f"tc={r['tc']['code']}/{r['tc']['secs']}s pc={r['pc']['code']}/{r['pc']['secs']}s",
        f"hooks pc={len(pc)} tc={len(tc)} same={same}",
    )
