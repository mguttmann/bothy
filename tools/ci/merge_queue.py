# SPDX-License-Identifier: Apache-2.0
"""The merge queue of main: a repository ruleset and the variable that switches the CI stages.

docs/quality/gates.md, "How a pull request lands"; docs/decisions.d/CI-SPLIT-MERGE-QUEUE.md. With
the queue a pull request runs the fast plan of ci.yml (BOTHY_MERGE_QUEUE=true, stage `pr`) and the
queue runs the full suite once per batch (`merge_group`, stage `queue`). The required check stays
`ci-ok` of the branch protection of main, which applies to pull requests and merge groups alike.

`apply` creates or updates the ruleset RULESET_NAME and only then sets the variable, so pull
requests never run the fast plan without a queue behind them; `disable` sets the variable back to
`false` first and then deletes the ruleset. GitHub offers merge queues only to repositories owned
by an organization, so `apply` refuses a repository of a personal account before it changes
anything. `show` prints the ruleset, the branch protection of main and the variable as JSON; none
of them holds a secret.

Usage:
  merge_queue.py ruleset                 print the ruleset this script applies
  merge_queue.py apply --repo OWNER/NAME create or update the queue, then set the variable
  merge_queue.py disable --repo OWNER/NAME
                                         set the variable to false, then delete the queue
  merge_queue.py show --repo OWNER/NAME  print the ruleset, the branch protection and the variable

Exit codes: 0 done, 1 a GitHub call failed or the repository cannot have a merge queue.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from collections.abc import Callable
from typing import Any

RULESET_NAME = "main merge queue"
VARIABLE = "BOTHY_MERGE_QUEUE"
BRANCH = "main"
# Up to five pull requests per batch, merged with squash as before. Only the head of a group must
# pass (HEADGREEN), so one full run covers the batch; when it fails, GitHub drops the pull request
# at its head and rebuilds the groups behind it. Two
# full runs at a time: each holds two macOS jobs (apps (macOS), apps (iOS)) of the five that
# GitHub runs at once for a free account, and the run of each push to main needs two more. The
# longest job, test (server contract), may take 240 minutes; the rest of the timeout is the wait
# for a runner.
QUEUE_PARAMETERS: dict[str, Any] = {
    "merge_method": "SQUASH",
    "max_entries_to_merge": 5,
    "min_entries_to_merge": 1,
    "min_entries_to_merge_wait_minutes": 5,
    "max_entries_to_build": 2,
    "grouping_strategy": "HEADGREEN",
    "check_response_timeout_minutes": 360,
}

Api = Callable[..., Any]


class GitHubError(Exception):
    """A call of the GitHub API failed, or the repository cannot have a merge queue."""


def ruleset() -> dict[str, Any]:
    """Return the ruleset of the merge queue of the default branch."""
    return {
        "name": RULESET_NAME,
        "target": "branch",
        "enforcement": "active",
        "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
        "rules": [{"type": "merge_queue", "parameters": dict(QUEUE_PARAMETERS)}],
    }


def gh_api(method: str, path: str, body: dict[str, Any] | None = None) -> Any:
    """Call `gh api` and return the parsed JSON answer (None for an empty one)."""
    args = ["gh", "api", "-X", method, path]
    if body is not None:
        args += ["--input", "-"]
    done = subprocess.run(
        args,
        input=json.dumps(body) if body is not None else None,
        capture_output=True,
        text=True,
        check=False,
    )
    if done.returncode != 0:
        raise GitHubError(f"{method} {path}: {(done.stderr or done.stdout).strip()}")
    return json.loads(done.stdout) if done.stdout.strip() else None


def _ruleset_id(repo: str, api: Api) -> int | None:
    for item in api("GET", f"repos/{repo}/rulesets") or []:
        if item.get("name") == RULESET_NAME:
            return int(item["id"])
    return None


def _variable(repo: str, api: Api) -> str | None:
    try:
        return str(api("GET", f"repos/{repo}/actions/variables/{VARIABLE}")["value"])
    except GitHubError as exc:
        if "404" in str(exc):
            return None
        raise


def _set_variable(repo: str, api: Api, value: str) -> None:
    body = {"name": VARIABLE, "value": value}
    if _variable(repo, api) is None:
        api("POST", f"repos/{repo}/actions/variables", body)
    else:
        api("PATCH", f"repos/{repo}/actions/variables/{VARIABLE}", body)


def apply(repo: str, api: Api = gh_api) -> None:
    """Create or update the merge queue of `repo`, then switch pull requests to the fast plan.

    Raises:
        GitHubError: the repository belongs to a personal account, or a call failed; the
            variable stays as it was then.
    """
    owner_type = (api("GET", f"repos/{repo}").get("owner") or {}).get("type")
    if owner_type != "Organization":
        raise GitHubError(
            f"{repo} belongs to a {owner_type or 'unknown'} account; GitHub offers merge queues"
            " only to repositories owned by an organization"
        )
    existing = _ruleset_id(repo, api)
    if existing is None:
        api("POST", f"repos/{repo}/rulesets", ruleset())
    else:
        api("PUT", f"repos/{repo}/rulesets/{existing}", ruleset())
    _set_variable(repo, api, "true")


def disable(repo: str, api: Api = gh_api) -> None:
    """Switch pull requests back to the full plan, then delete the merge queue of `repo`."""
    if _variable(repo, api) is not None:
        _set_variable(repo, api, "false")
    existing = _ruleset_id(repo, api)
    if existing is not None:
        api("DELETE", f"repos/{repo}/rulesets/{existing}")


def show(repo: str, api: Api = gh_api) -> dict[str, Any]:
    """Return the ruleset, the branch protection of main and the variable, without secrets."""
    existing = _ruleset_id(repo, api)
    stored = api("GET", f"repos/{repo}/rulesets/{existing}") if existing is not None else None
    protection = api("GET", f"repos/{repo}/branches/{BRANCH}/protection") or {}
    checks = protection.get("required_status_checks") or {}

    def enabled(key: str) -> bool | None:
        value = protection.get(key)
        return bool(value.get("enabled")) if isinstance(value, dict) else None

    return {
        "ruleset": None
        if stored is None
        else {
            key: stored.get(key)
            for key in ("id", "name", "target", "enforcement", "conditions", "rules")
        },
        "branch_protection": {
            "required_status_checks": checks.get("contexts"),
            "strict": checks.get("strict"),
            "enforce_admins": enabled("enforce_admins"),
            "required_linear_history": enabled("required_linear_history"),
            "allow_force_pushes": enabled("allow_force_pushes"),
            "allow_deletions": enabled("allow_deletions"),
            "required_conversation_resolution": enabled("required_conversation_resolution"),
        },
        "variable": {VARIABLE: _variable(repo, api)},
    }


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; returns the exit code."""
    parser = argparse.ArgumentParser(description="The merge queue of main.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("ruleset")
    for name in ("apply", "disable", "show"):
        sub.add_parser(name).add_argument("--repo", required=True)
    args = parser.parse_args(argv)
    if args.cmd == "ruleset":
        print(json.dumps(ruleset(), indent=2))
        return 0
    try:
        if args.cmd == "apply":
            apply(args.repo, gh_api)
        elif args.cmd == "disable":
            disable(args.repo, gh_api)
        print(json.dumps(show(args.repo, gh_api), indent=2))
    except GitHubError as exc:
        print(f"merge_queue: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
