# SPDX-License-Identifier: Apache-2.0
"""Pre-push gate: run the gates of the parts a push touches (docs/quality/push-discipline.md).

The pushed diff is every file that differs between the pushed commit and its merge base with
`origin/main`, the same range a pull request shows. The component map of CI
(`tools/ci/components.py changed`) turns it into `COMPONENTS`: the Go, Dart, SDK and plugin
gates run for the touched components only, exactly as the `gates` job of CI selects them. A
shared input (Makefile, tool locks and tools, the API contract, workflows, root files) maps to
`all` there, and then this gate runs the full `make check`.

Two further switches narrow the repository-wide gates:

- `AREAS=tools` runs format, lint, type check, tests and coverage of the Python tools under
  `tools/`. The tools tests read many files of the repository (workflows, the Makefile, docs,
  manifests), so the area is on for every changed file that is not source code of a component;
  only a push of component source files alone (for example Go or Dart files) leaves it out.
- `LINT_FROM_REF` and `LINT_TO_REF` limit the file hygiene hooks of `make lint` to the changed
  files.

Every other gate of `make check` runs on every push: commitlint, gitleaks, denylist, semgrep,
the docs gates and the supply-chain scans. CI keeps running its full selection; this gate only
decides what runs before the push. `PUSH_FULL=1` forces the full `make check`.

Usage:
  push_plan.py plan --files <path>...      print the make arguments for these files
  push_plan.py run [--dry-run]             plan the pushed diff and run make (the hook)
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

# tools/ci is no package; the component map of CI is read from there.
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ci"))
import components

# The ref the pushed diff is measured against; a pull request compares with main as well.
BASE_REF = "origin/main"
ZERO_SHA = "0" * 40
# Source files of a component: a push of only these leaves the tools gates out, except the few
# tools tests that read component sources (TOOLS_SOURCE_TESTS in the Makefile), which `make test`
# runs then. Manifests, lock files, configuration, test data and docs of a component are not on
# the list, because tools tests read some of them (go.mod, Dockerfile, .golangci.yml).
COMPONENT_DIRS = ("services/", "apps/", "packages/", "sdk/", "plugins/")
SOURCE_SUFFIXES = (".go", ".dart", ".ts", ".tsx", ".kt", ".swift", ".py")
# git exports these to hooks; make and the tools it starts must see the repository as CI does
# (docs/DECISIONS.md D-M4-05-6).
HOOK_VARIABLES = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX")


@dataclass
class Plan:
    """What `make check` runs for a push: full, or narrowed by components and areas."""

    full: bool
    reason: str
    components: list[str] = field(default_factory=list)
    areas: list[str] = field(default_factory=list)

    def make_args(self, base: str = "", head: str = "") -> list[str]:
        """Return the arguments of `make` for this plan (lint limited to base..head)."""
        if self.full:
            return ["check"]
        args = [
            "check",
            f"COMPONENTS={components.format_for_make(self.components)}",
            f"AREAS={' '.join(self.areas) or 'none'}",
        ]
        if base and head:
            args += [f"LINT_FROM_REF={base}", f"LINT_TO_REF={head}"]
        return args


def is_component_source(path: str) -> bool:
    """Return whether path is a source file of a component that no tools test reads."""
    return path.startswith(COMPONENT_DIRS) and path.endswith(SOURCE_SUFFIXES)


def plan(files: list[str]) -> Plan:
    """Return the plan for the changed files of a push."""
    selected = components.changed(files)
    if selected == ["all"]:
        shared = next((f for f in files if components.changed([f]) == ["all"]), "a shared input")
        return Plan(full=True, reason=f"{shared} is a shared input")
    areas = ["tools"] if any(not is_component_source(f) for f in files) else []
    reason = "components " + components.format_for_make(selected)
    return Plan(full=False, reason=reason, components=selected, areas=areas)


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout.strip()


def pushed_range(repo: Path) -> tuple[str, str] | None:
    """Return (merge base with origin/main, pushed commit), or None when there is no base."""
    ref = os.environ.get("PRE_COMMIT_TO_REF") or "HEAD"
    if ref == ZERO_SHA:
        return None
    head = _git(repo, "rev-parse", "--verify", f"{ref}^{{commit}}")
    try:
        base = _git(repo, "merge-base", BASE_REF, head)
    except subprocess.CalledProcessError:
        return None
    return base, head


def push_plan(repo: Path) -> tuple[Plan, str, str]:
    """Return the plan of the pushed diff with its range ("" when the full check runs)."""
    if os.environ.get("PUSH_FULL") == "1":
        return Plan(full=True, reason="PUSH_FULL=1"), "", ""
    if os.environ.get("PRE_COMMIT_REMOTE_BRANCH", "").startswith("refs/tags/"):
        return Plan(full=True, reason="a tag push"), "", ""
    rng = pushed_range(repo)
    if rng is None:
        return Plan(full=True, reason=f"no merge base with {BASE_REF}"), "", ""
    base, head = rng
    files = [
        f
        for f in _git(repo, "diff", "--name-only", "--no-renames", f"{base}...{head}").splitlines()
        if f
    ]
    if not files:
        return Plan(full=True, reason=f"no difference to {BASE_REF}"), "", ""
    return plan(files), base, head


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; returns the process exit code."""
    parser = argparse.ArgumentParser(description="Gates of the parts a push touches.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_plan = sub.add_parser("plan")
    p_plan.add_argument("--files", nargs="+", required=True)
    p_run = sub.add_parser("run")
    p_run.add_argument("--repo", type=Path, default=Path.cwd())
    p_run.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    if args.cmd == "plan":
        print(" ".join(plan(args.files).make_args()))
        return 0
    the_plan, base, head = push_plan(args.repo)
    make_args = the_plan.make_args(base, head)
    print(f"push plan: {the_plan.reason}: make {' '.join(make_args)}", flush=True)
    if args.dry_run:
        return 0
    env = {k: v for k, v in os.environ.items() if k not in HOOK_VARIABLES}
    return subprocess.run(["make", *make_args], cwd=args.repo, env=env, check=False).returncode


if __name__ == "__main__":
    sys.exit(main())
