# SPDX-License-Identifier: Apache-2.0
"""Prerequisites of the server gates: go, a Docker-compatible runtime and the tool pins.

Every server gate (`codegen-check`, `arch`, `migrations-lint`, `perf`) and the Go branch of
`make test` call this first, as soon as a Go module is tracked under `services/`. A
missing prerequisite is an error, never a skip (docs/spec/23-quality.md R-QA-02):
integration and load tests need a Docker-compatible runtime (docs/PLAN.md P-09), and a
machine that cannot run them must not report a partly green server check. All problems
are collected before the tool exits, so one run names every missing piece.

Usage: go_prereqs.py <gate name>

Exit codes: 0 everything present (no output), 1 one line per problem on stderr, 2 usage
error.
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any

# The Makefile runs this file as a script, so Python puts only tools/ci on sys.path; the
# pin reader lives in tools/bin (pytest finds it through its pythonpath setting instead).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "bin"))

import fetch_tools

TOOLS = ("go-arch-lint", "sqlc")
"""Pinned release binaries the server gates call (tools/bin/tools.lock.json)."""

DOCKER_TIMEOUT_S = 30
"""Seconds `docker info` may take before the runtime counts as unreachable."""

LOCK_NAME = "tools/bin/tools.lock.json"

Ensure = Callable[[fetch_tools.Lock, str, str, Path], Path]


def _docker_reachable(run: Callable[..., Any]) -> bool:
    """True if `docker info` succeeds; a missing binary or a timeout counts as failure."""
    try:
        result = run(
            ["docker", "info"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=DOCKER_TIMEOUT_S,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return False
    return bool(result.returncode == 0)


def _pin_problems(gate: str, ensure: Ensure, lock_path: Path) -> list[str]:
    """One line per pinned tool that is not pinned for this platform or cannot be fetched.

    Any failure while reading the lock or fetching a pin (a malformed lock, a network error,
    a truncated download) becomes a problem line, so the lines collected before it are
    still printed and the gate fails with a message instead of a traceback.
    """
    try:
        lock = fetch_tools.load_lock(lock_path)
        plat = fetch_tools.platform_key()
        pinned = {tool: lock["tools"][tool]["assets"] for tool in TOOLS if tool in lock["tools"]}
    except Exception as exc:  # any unreadable lock is a missing pin
        return [f"{gate}: {tool}: {exc}" for tool in TOOLS]
    problems: list[str] = []
    for tool in TOOLS:
        if tool not in pinned:
            problems.append(f"{gate}: {tool} is not pinned in {LOCK_NAME}")
        elif plat not in pinned[tool]:
            problems.append(f"{gate}: {tool} has no pin for {plat} in {LOCK_NAME}")
        else:
            try:
                ensure(lock, tool, plat, fetch_tools.DEFAULT_CACHE)
            except Exception as exc:  # a pin that cannot be fetched is missing
                problems.append(f"{gate}: {tool}: {exc}")
    return problems


def check(
    gate: str,
    *,
    which: Callable[[str], str | None] = shutil.which,
    run: Callable[..., Any] = subprocess.run,
    ensure: Ensure = fetch_tools.ensure,
    lock_path: Path = fetch_tools.DEFAULT_LOCK,
) -> list[str]:
    """Collect every missing prerequisite of a server gate.

    Checks, in this order: `go` on PATH, a reachable Docker-compatible runtime
    (`docker info`), and the pins of `TOOLS`, which `ensure` downloads and verifies on
    first use.

    Args:
        gate: gate name, the prefix of every problem line (for example `arch`).
        which: PATH lookup (injected by tests).
        run: process runner with the signature of `subprocess.run` (injected by tests).
        ensure: pinned-binary fetcher with the signature of `fetch_tools.ensure`.
        lock_path: the lock file of the release binaries.

    Returns:
        One line per problem, empty when everything is present.

    Example:
        >>> check("arch", which=lambda name: None)[0]
        'arch: go is not on PATH; install the version named in services/server/go.mod ...'
    """
    problems: list[str] = []
    if which("go") is None:
        problems.append(
            f"{gate}: go is not on PATH; install the version named in "
            "services/server/go.mod (CONTRIBUTING.md)"
        )
    if not _docker_reachable(run):
        problems.append(
            f'{gate}: no Docker-compatible runtime is reachable ("docker info" failed); '
            "integration and load tests need one (docs/PLAN.md P-09)"
        )
    problems.extend(_pin_problems(gate, ensure, lock_path))
    return problems


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(
        prog="go_prereqs.py", description="Prerequisites of the server gates."
    )
    parser.add_argument("gate", help="gate name, printed in front of every problem")
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    problems = check(args.gate)
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
