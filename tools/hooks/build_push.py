# SPDX-License-Identifier: Apache-2.0
"""Pre-push gate selection: only the fast gates for a push to a `build/**` branch.

A push to `build/<ticket>` starts `.github/workflows/build-verify.yml`, which runs every gate
of the required CI on hosted runners (docs/quality/push-discipline.md, "Build branches"). Before
such a push the hook runs only the gates that keep private or broken content off the remote
and are quick:

- `denylist`, `gitleaks` and `commitlint`, as in `make check`;
- `codegen-check` when a generator input changed: a gate component or a file outside the
  components that the Python generators of `make codegen-check` read (CODEGEN_INPUTS);
- `lint` of the touched components (`tools/ci/components.py changed`, the map CI uses).

The pushed diff runs from the merge base with `origin/main` to the pushed commit; without
`origin/main` every component is selected. Every other ref (and a push pre-commit reports
without a remote branch) runs the command given after `--` unchanged, today `make check`.

pre-commit names one ref per push (`PRE_COMMIT_REMOTE_BRANCH`, the first ref that is not a
deletion), so a single `git push` of a build branch together with another branch gets the
fast gates only; push other branches in a push of their own.

Usage:
  build_push.py [--repo <dir>] -- <command>...
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path

# tools/ci is no package; the component map of CI is read from there.
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ci"))
import components

BUILD_PREFIX = "build/"
BASE_REF = "origin/main"
# Files outside every gate component that a generator of `make codegen-check` reads; the inputs
# inside components and the API contract select a component (tools/ci/components.py).
CODEGEN_INPUTS = ("docs/operations/configuration.md",)
FAST_GATES = ("denylist", "gitleaks", "commitlint")
# git exports these to hooks; make and the tools it starts must see the repository as CI does
# (docs/DECISIONS.md D-M4-05-6).
HOOK_VARIABLES = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX")


def is_build_ref(ref: str) -> bool:
    """Return whether `ref` (full or short branch name) is a branch `build/<name>`."""
    name = ref.removeprefix("refs/heads/")
    if ref.startswith("refs/") and name == ref:
        return False
    return name.startswith(BUILD_PREFIX) and len(name) > len(BUILD_PREFIX)


def _clean_env() -> dict[str, str]:
    return {k: v for k, v in os.environ.items() if k not in HOOK_VARIABLES}


def _git(repo: Path, *args: str) -> subprocess.CompletedProcess[str]:
    # git with fixed arguments.
    return subprocess.run(  # nosec B603 B607
        ["git", "-C", str(repo), *args],
        capture_output=True,
        text=True,
        env=_clean_env(),
        check=False,
    )


def changed_files(repo: Path, head: str) -> list[str] | None:
    """Return the files changed from the merge base with origin/main to `head`, or None."""
    base = _git(repo, "merge-base", BASE_REF, head)
    if base.returncode != 0:
        return None
    diff = _git(repo, "diff", "--name-only", base.stdout.strip(), head)
    if diff.returncode != 0:
        return None
    return [line for line in diff.stdout.splitlines() if line]


def fast_make_args(repo: Path, head: str) -> list[str]:
    """Return the make arguments of the fast gates for a push of `head` to a build branch."""
    files = changed_files(repo, head)
    selected = ["all"] if files is None else components.changed(files)
    codegen = bool(selected) or any(path in CODEGEN_INPUTS for path in files or [])
    targets = [*FAST_GATES, *(["codegen-check"] if codegen else []), "lint"]
    return [*targets, f"COMPONENTS={components.format_for_make(selected)}"]


def main(argv: list[str] | None = None) -> int:
    """Run the fast gates for a build branch, else the given command; returns its exit code."""
    parser = argparse.ArgumentParser(description="Select the pre-push gates by pushed ref.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("a command after -- is required")
    ref = os.environ.get("PRE_COMMIT_REMOTE_BRANCH", "")
    if not is_build_ref(ref):
        # The hook's own command from .pre-commit-config.yaml, unchanged.
        return subprocess.run(command, check=False).returncode  # nosec B603
    head = (
        os.environ.get("PRE_COMMIT_TO_REF") or os.environ.get("PRE_COMMIT_LOCAL_BRANCH") or "HEAD"
    )
    cmd = ["make", *fast_make_args(args.repo, head)]
    print(
        f"build push: {ref} runs the fast gates ({' '.join(cmd)}); "
        ".github/workflows/build-verify.yml runs every gate on the pushed commit"
    )
    return subprocess.run(cmd, cwd=args.repo, env=_clean_env(), check=False).returncode  # nosec B603


if __name__ == "__main__":
    sys.exit(main())
