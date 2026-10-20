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

Jobs a change may not need (components.CONDITIONAL_JOBS) run only when the plan lists them. ci-ok
gets the plan of the same commit in CI_JOBS (its own step `jobs`, the same step as in the job
plan) and accepts a skipped job only when the plan left it out; a job that ran must pass either
way. A missing, empty or unknown plan fails (docs/quality/gates.md, "Jobs a change needs").

`ci_ok.py needs` is the same check for build-verify-ok of build-verify.yml, which has `needs`:
RESULTS is `toJSON(needs)`, CI_JOBS the jobs output of its plan.

Exit codes: 0 all required jobs passed, 1 a required job failed.
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

import components

OWN_NAME = "ci-ok"
SUCCESS = frozenset({"success"})


@dataclass(frozen=True)
class Rule:
    """At least one job of `names` must end with a conclusion in `ok`.

    `job` is the id of a conditional job (components.CONDITIONAL_JOBS): it may also end
    skipped, but only when the plan did not list it.
    """

    names: tuple[str, ...]
    ok: frozenset[str]
    job: str = ""

    def accepted(self, needed: frozenset[str]) -> frozenset[str]:
        """Return the conclusions that meet the rule for the jobs the plan needs."""
        if self.job and self.job not in needed:
            return self.ok | {"skipped"}
        return self.ok


RULES = (
    Rule(("plan",), SUCCESS),
    # `make gates` in parallel jobs (docs/quality/gates.md, "CI jobs"); the test jobs run when the
    # plan needs them ("Jobs a change needs").
    Rule(("gates",), SUCCESS),
    Rule(("gates (docs and contracts)",), SUCCESS),
    Rule(("gates (security and supply chain)",), SUCCESS),
    Rule(("test (tools)",), SUCCESS),
    Rule(("test (server)",), SUCCESS, "test-server"),
    Rule(("test (apps)",), SUCCESS, "test-apps"),
    Rule(("test (SDKs and plugins)",), SUCCESS, "test-sdk-plugins"),
    Rule(("denylist (private list)", "denylist (generic rules, fork)"), SUCCESS),
    # Skipped without the secrets (fork or bot pull request, D-DEPS-3), on main without
    # SONAR_MAIN_ANALYSIS, and after a failed or cancelled test job, which fails its own rule.
    Rule(("sonar",), frozenset({"success", "skipped"})),
    Rule(("postgres-image (amd64)",), SUCCESS, "postgres-image-amd64"),
    Rule(("postgres-image (arm64)",), SUCCESS, "postgres-image-arm64"),
    Rule(("apps (macOS)",), SUCCESS, "apps-macos"),
    Rule(("apps (Windows)",), SUCCESS, "apps-windows"),
    # The plan leaves them out while the repository is private (D-M5-01-5).
    Rule(("apps (Android API 26)",), SUCCESS, "apps-android-26"),
    Rule(("apps (Android API 36)",), SUCCESS, "apps-android-36"),
)


def parse_jobs(value: str) -> frozenset[str]:
    """Return the job ids of a plan's `jobs` output (`none` for no conditional job).

    Raises:
        ValueError: the value is empty or names an unknown job, so a missing plan fails.
    """
    names = value.split()
    if names == ["none"]:
        return frozenset()
    if not names:
        raise ValueError("CI_JOBS is empty: the plan of this run is missing")
    unknown = sorted(set(names) - set(components.CONDITIONAL_JOBS))
    if unknown:
        raise ValueError(f"CI_JOBS names unknown jobs: {', '.join(unknown)}")
    return frozenset(names)


def check_needs(results: dict[str, str], needed: frozenset[str]) -> list[str]:
    """Return the problems of a run from `needs` results (job id -> result), empty when green.

    The plan must succeed and every other job pass; a conditional job may be skipped only
    when the plan did not list it.
    """
    problems = [] if "plan" in results else ["plan: missing"]
    for job, result in sorted(results.items()):
        if result == "success":
            continue
        if result == "skipped" and job in components.CONDITIONAL_JOBS and job not in needed:
            continue
        if result == "skipped" and job in needed:
            problems.append(f"{job}: skipped, but the plan needs it")
        else:
            problems.append(f"{job}: {result}")
    return problems


def evaluate(jobs: list[dict[str, Any]], runner: str, needed: frozenset[str]) -> tuple[str, str]:
    """Return ("pass" | "fail" | "wait", summary) for the jobs of the run and the plan."""
    candidates = [
        j
        for j in jobs
        if (j["name"] == OWN_NAME or j["name"].endswith(f" / {OWN_NAME}"))
        and j["status"] != "completed"
    ]
    own = [j for j in candidates if j.get("runner_name") == runner]
    if not own and len(candidates) == 1:
        own = candidates
    if not own:
        return "wait", "own job not listed yet"
    prefix = own[0]["name"][: -len(OWN_NAME)]
    mine = [j["name"][len(prefix) :] for j in jobs if j["name"].startswith(prefix)]
    required = {n for rule in RULES for n in rule.names}
    duplicates = sorted({n for n in mine if n in required and mine.count(n) > 1})
    if duplicates:
        return "fail", f"duplicate job names: {', '.join(duplicates)}"
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
        ok = rule.accepted(needed)
        if any(j and j["status"] == "completed" and j["conclusion"] in ok for j in found):
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
    needed: frozenset[str],
    *,
    sleep: Callable[[float], None] = time.sleep,
    interval: float = 15.0,
) -> int:
    """Poll `fetch` until the result is decided; return the exit code."""
    last = ""
    while True:
        try:
            verdict, summary = evaluate(fetch(), runner, needed)
        except Exception as exc:  # every reading error is retried; the job timeout ends it
            status = f" HTTP {exc.code}" if isinstance(exc, urllib.error.HTTPError) else ""
            print(
                f"ci-ok: reading the jobs failed ({type(exc).__name__}{status}); retrying",
                flush=True,
            )
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


def main(argv: list[str] | None = None) -> int:
    """Entry point of the ci-ok job, or of build-verify-ok with the argument `needs`."""
    args = sys.argv[1:] if argv is None else argv
    needed = parse_jobs(os.environ.get("CI_JOBS", ""))
    if args == ["needs"]:
        results = {
            job: str(value["result"]) for job, value in json.loads(os.environ["RESULTS"]).items()
        }
        problems = check_needs(results, needed)
        for problem in problems:
            print(f"build-verify-ok: {problem}")
        if problems:
            return 1
        print("build-verify-ok: every job the plan needs passed, the others passed or were skipped")
        return 0
    interval = float(os.environ.get("CI_OK_POLL_SECONDS", "15"))  # pragma: no cover
    return run(
        _fetch_from_api, os.environ["RUNNER_NAME"], needed, interval=interval
    )  # pragma: no cover


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
