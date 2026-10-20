# SPDX-License-Identifier: Apache-2.0
"""The result jobs ci-ok of ci.yml and build-verify-ok of build-verify.yml.

Both are the last job of their run: they need every other job and run with `always()`, so they
start only when the others have ended, hold no runner while the others run, and report on every
run, also after a failed, skipped or cancelled job. `ci_ok.py needs` reads the results of the
`needs` (RESULTS, `toJSON(needs)`) and the plan of the run (CI_JOBS, the `jobs` output of the
job plan) and fails when a job did not pass:

- the plan must succeed, and the plan and every conditional job (components.CONDITIONAL_JOBS)
  must be among the results;
- a conditional job may end skipped only when the plan did not list it; a job that ran must pass,
  listed or not;
- `sonar` (MAY_SKIP) may end skipped: without the secrets, on main without SONAR_MAIN_ANALYSIS,
  and after a failed or cancelled test job, which fails through that job;
- of the deny-list pair (ONE_OF) exactly the one that ran must pass; both skipped fails;
- every other job must succeed;
- a missing, empty or unknown plan fails (docs/quality/gates.md, "Jobs a change needs").

A job with `needs` gets its check run only when the jobs it needs have ended, so after an edit of
the pull request title or body the older green ci-ok of the same head commit would stay the latest
result for the whole new run. The first step of the plan job of ci.yml therefore creates a check
run `ci-ok` with status `in_progress` for the head commit, and ci-ok completes it with its own
conclusion (docs/quality/push-discipline.md, "Pull request title and body").

Exit codes: 0 every job passed as the plan requires, 1 otherwise.
"""

from __future__ import annotations

import json
import os
import sys

import components

# Jobs that may end skipped whatever the plan says.
MAY_SKIP = frozenset({"sonar"})
# Groups of jobs of which exactly one runs; that one must pass.
ONE_OF: tuple[frozenset[str], ...] = (frozenset({"denylist", "denylist-fork"}),)
# Jobs that must be among the results of every run.
REQUIRED = ("plan", *components.CONDITIONAL_JOBS)


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


def _skip_allowed(job: str, results: dict[str, str], needed: frozenset[str]) -> bool:
    if job in components.CONDITIONAL_JOBS:
        return job not in needed
    if job in MAY_SKIP:
        return True
    return any(
        job in group and any(results.get(other) == "success" for other in group - {job})
        for group in ONE_OF
    )


def check_needs(results: dict[str, str], needed: frozenset[str]) -> list[str]:
    """Return the problems of a run from `needs` results (job id -> result), empty when green."""
    problems = [f"{job}: missing" for job in REQUIRED if job not in results]
    for job, result in sorted(results.items()):
        if result == "success":
            continue
        if result == "skipped" and _skip_allowed(job, results, needed):
            continue
        if result == "skipped" and job in needed:
            problems.append(f"{job}: skipped, but the plan needs it")
        else:
            problems.append(f"{job}: {result}")
    return problems


def main(argv: list[str] | None = None) -> int:
    """Entry point of ci-ok and build-verify-ok (`ci_ok.py needs`); returns the exit code."""
    args = sys.argv[1:] if argv is None else argv
    if args != ["needs"]:
        raise SystemExit("usage: ci_ok.py needs")
    label = os.environ.get("GITHUB_JOB") or "ci-ok"
    results = {
        job: str(value["result"]) for job, value in json.loads(os.environ["RESULTS"]).items()
    }
    try:
        needed = parse_jobs(os.environ.get("CI_JOBS", ""))
    except ValueError as exc:
        print(f"{label}: {exc}")
        # Name the failed jobs too, as if the plan had needed every job.
        for problem in check_needs(results, frozenset(components.CONDITIONAL_JOBS)):
            print(f"{label}: {problem}")
        return 1
    problems = check_needs(results, needed)
    for problem in problems:
        print(f"{label}: {problem}")
    if problems:
        return 1
    print(f"{label}: every job the plan needs passed, the others passed or were skipped")
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
