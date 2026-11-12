# SPDX-License-Identifier: Apache-2.0
"""Report the failed runs that no pull request shows: scheduled runs and pushes to `main`.

A pull request shows the result of its own run. A scheduled run and the run of a push to `main`
show theirs to nobody: the platform suite of the host service and the nightly mobile E2E run
failed for weeks without a trace outside the list of runs (docs/decisions.d/AUD-misc-testing-01.md).
`report` (workflow red-runs.yml) reads the completed runs of the watched workflows of the last
hours and keeps one issue per workflow with the label LABEL:

- watched are the scheduled runs of every workflow with a `schedule` trigger and the pushes to
  `main` of MAIN_PUSH_WORKFLOWS of check_runners.py (`ci.yml`, the full suite of a change after
  its merge); both sets are read from `.github/workflows`, so a new scheduled workflow is
  watched without a change here. denylist-main.yml reports its own failed scan.
- a run is red when it ended as FAILED, or as cancelled with a job that ran and did not pass:
  GitHub cancels a job that hits its `timeout-minutes` and then lists the whole run as
  cancelled, also when another job failed. A run cancelled before any job started (a newer
  push took its place) is not red.
- each red run gets one comment with its jobs that did not pass and their steps that did not
  pass, as links to the job pages; no log text. The first one opens the issue "Red runs of
  <file>". The issue body lists the jobs, the most frequent first.
- the issue is closed when the newest watched run of the workflow passed and no run failed for
  AGE. The next red run opens a new issue.

Only the comments this workflow wrote count as reported (the hidden line of each names its
run), so a comment of someone else hides no run.

Usage:
  red_runs.py report --repo OWNER/NAME [--hours H] [--dry-run]
      --dry-run reads the runs and the issues and prints the gh commands that would write

Exit codes: 0 done, 1 an error (on stderr).
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import shlex
import sys
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import check_runners
import flakes
import yaml

SELF = Path(__file__).resolve()
WORKFLOWS = SELF.parents[2] / ".github" / "workflows"
# The tracking issues: one open issue per workflow with this label.
LABEL = "red-run"
# An issue is closed when the newest run passed and no run failed for this long.
AGE = dt.timedelta(days=7)
# The conclusions of a run that failed; `cancelled` counts only with a job that ran.
FAILED = frozenset({"failure", "timed_out", "startup_failure"})
CANCELLED = "cancelled"
# What `report` reads of a run and of a job (jq objects of the GitHub API).
RUN_FIELDS = "{id, html_url, event, head_branch, head_sha, conclusion, updated_at}"
JOB_FIELDS = "{name, conclusion, html_url, steps: [.steps[]? | {name, conclusion}]}"
# The hidden line of a comment of `report` with the run and its jobs (JSON).
_MARKER = re.compile(r"<!-- red-runs run=(\d+) (\{.*?\}) -->")
# An event or a conclusion as GitHub names them, and the start of a commit id.
_WORD = re.compile(r"^[a-z_]{1,30}$")
_SHA = re.compile(r"^[0-9a-f]{7,12}$")
_GITHUB_URL = re.compile(r"^https://github\.com/[A-Za-z0-9_./-]+$")
NO_JOB = "(no job ran)"


@dataclass(frozen=True)
class Red:
    """A reported red run: the jobs that did not pass and when the run ended."""

    jobs: tuple[str, ...]
    seen: dt.datetime


def watched(directory: Path = WORKFLOWS) -> dict[str, tuple[str, ...]]:
    """The watched workflow files with the events whose runs no pull request shows."""
    found: dict[str, tuple[str, ...]] = {}
    for path in sorted([*directory.glob("*.yml"), *directory.glob("*.yaml")]):
        doc = check_runners.load_yaml(path.read_text(encoding="utf-8"))
        triggers = check_runners._triggers(doc if isinstance(doc, dict) else {})
        events = [
            *(["schedule"] if "schedule" in triggers else []),
            *(["push"] if path.name in check_runners.MAIN_PUSH_WORKFLOWS else []),
        ]
        if events:
            found[path.name] = tuple(events)
    return found


def title(file: str) -> str:
    """The title of the tracking issue of the workflow `file`."""
    return f"Red runs of {file}"


def _label_args() -> list[str]:
    """Colour and description of LABEL from .github/labels.yml (the source of the labels)."""
    labels = yaml.safe_load(
        (SELF.parents[2] / ".github" / "labels.yml").read_text(encoding="utf-8")
    )
    [item] = [i for i in labels if i.get("name") == LABEL]
    return ["--color", str(item["color"]), "--description", str(item["description"])]


def not_passed(jobs: Iterable[dict[str, Any]]) -> list[dict[str, Any]]:
    """The jobs that ran and did not pass, each with only its steps that did not pass.

    A cancelled job counts when one of its steps failed or was cancelled; without such a step
    it never started.
    """
    out: list[dict[str, Any]] = []
    for job in jobs:
        conclusion = str(job.get("conclusion") or "")
        steps = [
            s
            for s in job.get("steps") or []
            if str(s.get("conclusion") or "") in FAILED | {CANCELLED}
        ]
        if conclusion in FAILED or (conclusion == CANCELLED and steps):
            out.append({**job, "steps": steps})
    return out


def marker(run: int, jobs: Sequence[str], seen: str) -> str:
    """The hidden line of a comment: the run, its jobs and its end; safe in an HTML comment."""
    payload = json.dumps({"jobs": list(jobs), "seen": seen}, sort_keys=True)
    payload = payload.replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
    return f"<!-- red-runs run={run} {payload} -->"


def parse_comments(comments: Iterable[tuple[str, str]]) -> dict[int, Red]:
    """The runs in the comments (body, created_at), by their hidden lines.

    A run counts as seen at the `seen` of its line, or when its comment was written.
    """
    found: dict[int, Red] = {}
    for text, created in comments:
        for match in _MARKER.finditer(text or ""):
            try:
                payload = json.loads(match[2])
            except ValueError:
                continue
            jobs = payload.get("jobs") if isinstance(payload, dict) else None
            if not isinstance(jobs, list):
                continue
            when = flakes._parse_time(payload.get("seen")) or flakes._parse_time(created)
            if when:
                found[int(match[1])] = Red(tuple(str(j)[: flakes._NAME_MAX] for j in jobs), when)
    return found


def comment(run: dict[str, Any], jobs: Sequence[dict[str, Any]], now: dt.datetime) -> str:
    """The comment for one red run: its jobs that did not pass with their steps.

    The run counts as seen when it ended, or `now` when the API names no such time.
    """
    rows = ["| Job | Result | Steps that did not pass |", "|---|---|---|"]
    for job in jobs:
        linked = _link(flakes._cell(job.get("name", "")), job.get("html_url"))
        steps = ", ".join(flakes._cell(s.get("name", "")) for s in job["steps"])
        rows.append(f"| {linked} | {_conclusion(job)} | {steps} |")
    if not jobs:
        rows.append(f"| {NO_JOB} | | |")
    seen = flakes._parse_time(run.get("updated_at")) or now
    head = (
        f"Run {_link(str(int(run['id'])), run.get('html_url'))} ({_word(run.get('event'))}, "
        f"{flakes._cell(run.get('head_branch') or '')}, {_sha(run.get('head_sha'))}): "
        f"{_conclusion(run)}\n\n"
    )
    names = [str(j.get("name", ""))[: flakes._NAME_MAX] for j in jobs]
    hidden = marker(int(run["id"]), names, seen.strftime("%Y-%m-%dT%H:%M:%SZ"))
    return head + "\n".join(rows) + "\n\n" + hidden + "\n"


def _link(text: str, url: Any) -> str:
    """`text` as a link to `url` when that is a page of GitHub, else `text` alone."""
    return f"[{text}]({url})" if _GITHUB_URL.match(str(url or "")) else text


def _word(value: Any) -> str:
    text = str(value or "")
    return text if _WORD.match(text) else "unknown"


def _sha(value: Any) -> str:
    text = str(value or "")[:12]
    return text if _SHA.match(text) else "unknown"


def _conclusion(item: dict[str, Any]) -> str:
    return _word(item.get("conclusion"))


def body(file: str, seen: dict[int, Red]) -> str:
    """The issue body: the jobs of the red runs, the most frequent and then the newest first."""
    count: dict[str, int] = {}
    last: dict[str, int] = {}
    for run, red in sorted(seen.items()):
        for job in red.jobs or (NO_JOB,):
            count[job] = count.get(job, 0) + 1
            last[job] = run
    rows = [
        f"| {count[job]} | {last[job]} | {seen[last[job]].seen.strftime('%Y-%m-%d')} | "
        f"{job if job == NO_JOB else flakes._cell(job)} |"
        for job in sorted(count, key=lambda j: (-count[j], -last[j], j))
    ]
    return (
        f"Runs of `.github/workflows/{file}` that failed where no pull request shows it: "
        'scheduled runs and pushes to `main` (docs/quality/gates.md, section "Red runs"). The '
        "workflow red-runs.yml adds one comment per failed run and keeps this list of the jobs "
        "that did not pass. It closes the issue when the newest run passed and no run failed "
        f"for {AGE.days} days; the next failed run opens a new one.\n\n"
        "| Failed runs | Last run | Last seen | Job |\n|---|---|---|---|\n" + "\n".join(rows) + "\n"
    )


def _dry(gh: flakes.Gh) -> flakes.Gh:
    """`gh` for a dry run: reads pass, every call that would write is printed instead."""

    def run(args: Sequence[str]) -> str:
        if args[0] == "api" or list(args[:2]) == ["issue", "list"]:
            return gh(args)
        print("dry run: gh " + shlex.join(args))
        return ""

    return run


def _runs(repo: str, file: str, event: str, gh: flakes.Gh, since: str = "") -> list[dict[str, Any]]:
    """The completed runs of `file` for `event`: created since `since`, or only the newest."""
    window = f"&created=>={since}&per_page=100" if since else "&per_page=1"
    path = f"repos/{repo}/actions/workflows/{file}/runs?status=completed&event={event}{window}"
    args = [
        "api",
        *(["--paginate"] if since else []),
        path,
        "--jq",
        f".workflow_runs[] | {RUN_FIELDS}",
    ]
    runs: list[dict[str, Any]] = flakes._json_lines(gh(args))
    return runs if since else runs[:1]


def _jobs(repo: str, run: int, gh: flakes.Gh) -> list[dict[str, Any]]:
    path = f"repos/{repo}/actions/runs/{run}/jobs?filter=latest&per_page=100"
    jobs: list[dict[str, Any]] = flakes._json_lines(
        gh(["api", "--paginate", path, "--jq", f".jobs[] | {JOB_FIELDS}"])
    )
    return not_passed(jobs)


def _own_comments(repo: str, number: int, gh: flakes.Gh) -> list[tuple[str, str]]:
    """(body, created_at) of the comments this workflow wrote on the issue `number`."""
    listed = gh(
        [
            "api",
            "--paginate",
            f"repos/{repo}/issues/{number}/comments",
            "--jq",
            ".[] | {body, user: .user.login, created_at}",
        ]
    )
    return [
        (str(c.get("body") or ""), str(c.get("created_at") or ""))
        for c in flakes._json_lines(listed)
        if c.get("user") == flakes.BOT
    ]


def report(
    repo: str,
    hours: int,
    gh: flakes.Gh,
    now: dt.datetime,
    dry_run: bool = False,
    directory: Path = WORKFLOWS,
) -> int:
    """Add the red runs of the last `hours` to the issues and close the ones that recovered."""
    gh = _dry(gh) if dry_run else gh
    since = (now - dt.timedelta(hours=hours)).strftime("%Y-%m-%dT%H:%M:%SZ")
    issues = json.loads(
        gh(
            [
                "issue",
                "list",
                "-R",
                repo,
                "--label",
                LABEL,
                "--state",
                "all",
                "--limit",
                "200",
                "--json",
                "number,title,state,body",
            ]
        )
    )
    added = 0
    for file, events in watched(directory).items():
        runs = [r for event in events for r in _runs(repo, file, event, gh, since)]
        candidates = [r for r in runs if str(r.get("conclusion")) in FAILED | {CANCELLED}]
        mine = [i for i in issues if i.get("title") == title(file)]
        number = next((int(i["number"]) for i in mine if i.get("state") == "OPEN"), None)
        if not candidates and number is None:
            continue
        # The runs of every issue of the workflow are reported; the open one lists its own.
        reported: set[int] = set()
        seen: dict[int, Red] = {}
        for issue in mine:
            parsed = parse_comments(_own_comments(repo, int(issue["number"]), gh))
            reported |= set(parsed)
            if int(issue["number"]) == number:
                seen = parsed
        fresh = 0
        for run in sorted(candidates, key=lambda r: int(r["id"])):
            if int(run["id"]) in reported:
                continue
            jobs = _jobs(repo, int(run["id"]), gh)
            if run.get("conclusion") == CANCELLED and not jobs:
                continue
            if number is None:
                gh(["label", "create", LABEL, "-R", repo, "--force", *_label_args()])
                made = gh(
                    [
                        "issue",
                        "create",
                        "-R",
                        repo,
                        "--title",
                        title(file),
                        "--label",
                        LABEL,
                        "--body",
                        body(file, {}),
                    ]
                )
                # A dry run prints the command and has no issue number to name.
                number = int(made.strip().rstrip("/").rsplit("/", 1)[1]) if made.strip() else 0
                mine.append({"number": number, "body": body(file, {})})
            text = comment(run, jobs, now)
            gh(["issue", "comment", str(number), "-R", repo, "--body", text])
            seen.update(parse_comments([(text, "")]))
            reported.add(int(run["id"]))
            fresh += 1
        if number is None:
            continue
        if fresh:
            where = f"issue #{number}" if number else "a new issue"
            print(f"red-runs: {fresh} failed run(s) added to {where} ({file})")
            added += fresh
        current = next((str(i.get("body") or "") for i in mine if int(i["number"]) == number), "")
        if seen and body(file, seen) != current:
            gh(["issue", "edit", str(number), "-R", repo, "--body", body(file, seen)])
        if not fresh:
            _close_recovered(repo, file, events, number, seen, now, gh)
    if not added:
        print("red-runs: no new failed run")
    return 0


def _close_recovered(
    repo: str,
    file: str,
    events: Sequence[str],
    number: int,
    seen: dict[int, Red],
    now: dt.datetime,
    gh: flakes.Gh,
) -> None:
    """Close the issue when the newest run passed and no run failed for AGE."""
    if any(now - red.seen <= AGE for red in seen.values()):
        return
    newest = [r for event in events for r in _runs(repo, file, event, gh)]
    if not newest:
        return
    last = max(newest, key=lambda r: int(r["id"]))
    if last.get("conclusion") != "success":
        return
    note = (
        f"Run {_link(str(int(last['id'])), last.get('html_url'))} passed and no watched run of "
        f"{file} failed in the last {AGE.days} days, so the workflow red-runs.yml closes the "
        "issue. The next failed run opens a new one."
    )
    gh(["issue", "close", str(number), "-R", repo, "--reason", "completed", "--comment", note])
    print(f"red-runs: issue #{number} closed, {file} passed again")


def main(argv: list[str] | None = None) -> int:
    """Entry point; returns the exit code."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    cmd = sub.add_parser("report")
    cmd.add_argument("--repo", required=True)
    cmd.add_argument("--hours", type=int, default=48)
    cmd.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    try:
        return report(args.repo, args.hours, flakes._gh, dt.datetime.now(dt.UTC), args.dry_run)
    except ValueError as exc:
        print(f"red-runs: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
