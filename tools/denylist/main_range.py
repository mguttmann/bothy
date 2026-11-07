# SPDX-License-Identifier: Apache-2.0
"""Commit range that .github/workflows/denylist-main.yml scans (DENYLIST_RANGE).

A push to main scans the pushed commits `before..after`; without a usable `before` (the
first push, a commit the clone does not have) every commit reachable from `after`, as
before. A manual run (workflow_dispatch) scans what a push run scans and nothing older: the
tree of main always, and commits only from the optional input `range`. It never scans the
whole history, whose old commits are public and accepted; a run without an input returns
`after..after`, an empty range.

The input is untrusted text from the run form. Only `BASE..TIP` with two lower case commit
ids (7 to 40 hex digits) that exist in the clone is accepted, so no option or revision
expression reaches git. Reads EVENT_NAME, BEFORE, AFTER and INPUT_RANGE from the
environment and prints the range. Exit codes: 0 range printed, 2 rejected input.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys
from collections.abc import Callable

ZERO = "0" * 40
_INPUT = re.compile(r"^([0-9a-f]{7,40})\.\.([0-9a-f]{7,40})$")


class RangeError(Exception):
    """The event or the requested range cannot be scanned safely."""


def scan_range(
    event: str, before: str, after: str, requested: str, exists: Callable[[str], bool]
) -> str:
    """Return the range for `make denylist`; `exists` tells whether a commit is in the clone."""
    if not after:
        raise RangeError("the commit of the run (AFTER) is missing")
    if event == "push":
        if before and before != ZERO and exists(before):
            return f"{before}..{after}"
        return after
    if event == "workflow_dispatch":
        text = requested.strip()
        if not text:
            return f"{after}..{after}"
        match = _INPUT.match(text)
        if not match:
            raise RangeError("the input range must be BASE..TIP with two commit ids")
        for sha in match.groups():
            if not exists(sha):
                raise RangeError(f"unknown commit {sha} in the input range")
        return text
    raise RangeError(f"event {event!r} is not scanned by denylist-main")


def commit_exists(sha: str) -> bool:
    """Return True if `sha` names a commit in the repository of the working directory."""
    result = subprocess.run(
        ["git", "cat-file", "-e", f"{sha}^{{commit}}"], capture_output=True, check=False
    )
    return result.returncode == 0


def main(argv: list[str] | None = None) -> int:
    """Print the range from the environment of the workflow step; 2 on rejected input."""
    del argv
    env = os.environ
    try:
        rev_range = scan_range(
            env.get("EVENT_NAME", ""),
            env.get("BEFORE", ""),
            env.get("AFTER", ""),
            env.get("INPUT_RANGE", ""),
            commit_exists,
        )
    except RangeError as exc:
        print(f"denylist-main: {exc}", file=sys.stderr)
        return 2
    print(rev_range)
    return 0


if __name__ == "__main__":
    sys.exit(main())
