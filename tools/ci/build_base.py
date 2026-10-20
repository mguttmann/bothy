# SPDX-License-Identifier: Apache-2.0
"""The base commit that build-verify.yml measures the diff of a build branch from.

docs/quality/push-discipline.md, "Build branches": a build team pushes `build/<ticket>` and may
push `build-base/<ticket>` right before it, pointing at the integration head the ticket branch
was built on. The plan of build-verify.yml (tools/ci/components.py ci-plan) then runs the jobs of
the files between the merge base of the pushed commit with that ref and the pushed commit, as a
pull request on that head would show them. Without the ref, or without common history with it,
the base is the merge base with `origin/main` as before; without any base every job runs.

The refs are read as `actions/checkout` with `fetch-depth: 0` leaves them: every branch of the
repository under `refs/remotes/origin/`.

Usage:
  build_base.py --branch build/<ticket> --head <sha> [--repo <dir>] [--github-output <file>]
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

BUILD_PREFIX = "build/"
BASE_PREFIX = "build-base/"
MAIN = "origin/main"


@dataclass(frozen=True)
class Base:
    """The base commit of the plan (empty: every job runs) and where it comes from."""

    sha: str
    source: str


def _merge_base(repo: Path, ref: str, head: str) -> str:
    """Return the merge base of `ref` and `head`, or "" without the ref or common history."""
    result = subprocess.run(
        ["git", "-C", str(repo), "merge-base", ref, head],
        capture_output=True,
        text=True,
        check=False,
    )
    return result.stdout.strip() if result.returncode == 0 else ""


def select(repo: Path, branch: str, head: str) -> Base:
    """Return the base of the plan for the pushed `branch` (`build/<ticket>`) at `head`.

    Raises:
        ValueError: `branch` is no build branch.
    """
    ticket = branch.removeprefix(BUILD_PREFIX)
    if not branch.startswith(BUILD_PREFIX) or not ticket:
        raise ValueError(f"{branch!r} is no build branch build/<ticket>")
    base_ref = f"origin/{BASE_PREFIX}{ticket}"
    sha = _merge_base(repo, f"refs/remotes/{base_ref}", head)
    if sha:
        return Base(sha, f"merge base with {base_ref}")
    sha = _merge_base(repo, f"refs/remotes/{MAIN}", head)
    if sha:
        return Base(sha, f"merge base with {MAIN}")
    return Base("", f"no merge base with {MAIN}")


def main(argv: list[str] | None = None) -> int:
    """Print the base and write `base=<sha>` to --github-output; returns the exit code."""
    parser = argparse.ArgumentParser(description="Base of the change plan of a build branch.")
    parser.add_argument("--branch", required=True)
    parser.add_argument("--head", required=True)
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args(argv)
    base = select(args.repo, args.branch, args.head)
    print(f"base: {base.sha or 'none'} ({base.source})")
    if args.github_output:
        with args.github_output.open("a", encoding="utf-8") as out:
            out.write(f"base={base.sha}\n")
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
