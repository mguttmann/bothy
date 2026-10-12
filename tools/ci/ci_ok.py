# SPDX-License-Identifier: Apache-2.0
"""The ci-ok job of ci.yml: wait for the other jobs of the run and combine their results.

ci-ok is the only required status check. It has no `needs`: a job with `needs` gets its
check run only when its dependencies are done, so after an edit of the pull request title or
body the older green ci-ok of the same head commit would stay the latest result for the
whole new run, and the edited text could be merged unchecked. Without `needs`, the ci-ok
check run is pending from the start of every run and blocks the merge until this script
decides.

The script reads the jobs of its own run through the REST API (`actions: read`), finds its
own job by the runner name (or as the only running ci-ok job) and so the prefix of a called
workflow (`gates / ` when release.yml calls ci.yml), and applies RULES to the jobs with
that prefix. A failure decides at once; success needs every rule to be met. API errors are
retried; the job's timeout ends a run that never decides (fail closed).

Exit codes: 0 all required jobs passed, 1 a required job failed.
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

OWN_NAME = "ci-ok"


@dataclass(frozen=True)
class Rule:
    """At least one job of `names` must end with a conclusion in `ok`."""

    names: tuple[str, ...]
    ok: frozenset[str]


RULES = (
    Rule(("plan",), frozenset({"success"})),
    Rule(("gates",), frozenset({"success"})),
    Rule(("denylist (private list)", "denylist (generic rules, fork)"), frozenset({"success"})),
    Rule(("sonar",), frozenset({"success", "skipped"})),
)


def evaluate(jobs: list[dict[str, Any]], runner: str) -> tuple[str, str]:
    """Return ("pass" | "fail" | "wait", summary) for the jobs of the run."""
    candidates = [j for j in jobs if j["name"].endswith(OWN_NAME) and j["status"] != "completed"]
    own = [j for j in candidates if j.get("runner_name") == runner]
    if not own and len(candidates) == 1:
        own = candidates
    if not own:
        return "wait", "own job not listed yet"
    prefix = own[0]["name"][: -len(OWN_NAME)]
    by_name = {j["name"][len(prefix) :]: j for j in jobs if j["name"].startswith(prefix)}
    states: list[str] = []
    decided_fail = False
    pending = False
    for rule in RULES:
        found = [by_name.get(n) for n in rule.names]
        states += [
            f"{n}={j['conclusion'] if j and j['status'] == 'completed' else 'pending'}"
            for n, j in zip(rule.names, found, strict=True)
        ]
        if any(j and j["status"] == "completed" and j["conclusion"] in rule.ok for j in found):
            continue
        if all(j and j["status"] == "completed" for j in found):
            decided_fail = True
        else:
            pending = True
    summary = " ".join(states)
    if decided_fail:
        return "fail", summary
    return ("wait" if pending else "pass"), summary


def run(
    fetch: Callable[[], list[dict[str, Any]]],
    runner: str,
    *,
    sleep: Callable[[float], None] = time.sleep,
    interval: float = 15.0,
) -> int:
    """Poll `fetch` until the result is decided; return the exit code."""
    last = ""
    while True:
        try:
            verdict, summary = evaluate(fetch(), runner)
        except (OSError, ValueError, KeyError) as exc:
            print(f"ci-ok: reading the jobs failed ({type(exc).__name__}); retrying", flush=True)
        else:
            if summary != last:
                print(f"ci-ok: {summary}", flush=True)
                last = summary
            if verdict != "wait":
                print(f"ci-ok: {verdict}", flush=True)
                return 0 if verdict == "pass" else 1
        sleep(interval)


def _fetch_from_api() -> list[dict[str, Any]]:  # pragma: no cover - needs the GitHub API
    api = os.environ.get("GITHUB_API_URL", "https://api.github.com")
    url = (
        f"{api}/repos/{os.environ['GITHUB_REPOSITORY']}/actions/runs/"
        f"{os.environ['GITHUB_RUN_ID']}/jobs?filter=latest&per_page=100"
    )
    if not url.startswith("https://"):
        raise ValueError("GITHUB_API_URL must be an https URL")
    request = urllib.request.Request(  # noqa: S310 - https URL checked above
        url,
        headers={
            "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:  # noqa: S310  # nosec B310
        jobs = json.load(response)["jobs"]
    if not isinstance(jobs, list):
        raise ValueError("unexpected jobs payload")
    return jobs


def main() -> int:  # pragma: no cover - entry point on the runner
    """Entry point of the ci-ok job."""
    interval = float(os.environ.get("CI_OK_POLL_SECONDS", "15"))
    return run(_fetch_from_api, os.environ["RUNNER_NAME"], interval=interval)


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
