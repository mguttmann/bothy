# SPDX-License-Identifier: Apache-2.0
"""Gradle lock files in the advisory and license gates: which of their packages ship.

A Gradle lock file lists every package a build resolves, each with the configurations that
resolve it (`group:artifact:version=configuration,configuration`; the line `empty=` names the
configurations without a package). Most of them never reach a device: the test platform of the
Android Gradle Plugin (`_internal-unified-test-platform-*`), lint, the Kotlin compiler, the
classpaths of unit and instrumented tests, the debug and profile variants and the build script
classpath. OSV-Scanner reads the file as a flat list of packages, so `make osv`
(tools/ci/advisories.py) and `make licenses` (tools/ci/dep_licenses.py) judge a package of a
Gradle lock file only when it ships (docs/decisions.d/AUD-root-01.md):

- on the runtime classpath of a release build: `releaseRuntimeClasspath`, with product flavors
  `<flavor>ReleaseRuntimeClasspath`, and `runtimeClasspath` of a module without build variants;
- as `coreLibraryDesugaring`, the library the build compiles into the app.

A lock file of a build script classpath (`buildscript-gradle.lockfile`) ships nothing. Any other
lock file that names none of these configurations, neither for a package nor in its `empty=`
line, fails: with another variant layout the gates would read nothing of the file.

OSV-Scanner picks its parser by the file name and knows `gradle.lockfile` and
`buildscript-gradle.lockfile`; `scan_arg` names the format for a variant such as
`gradle-firebase.lockfile`.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

# (group:artifact, version), the name and version OSV-Scanner reports for a Maven package.
Package = tuple[str, str]

GRADLE_LOCK = "gradle.lockfile"
BUILDSCRIPT_LOCK = "buildscript-gradle.lockfile"
_SUFFIX = ".lockfile"
_SHIPS = re.compile(r"^(?:\w*[rR]elease)?[rR]untimeClasspath$")
_DESUGARING = "coreLibraryDesugaring"


class LockError(Exception):
    """A Gradle lock file cannot be read or names no configuration that ships."""


@dataclass(frozen=True)
class Lock:
    """The entries of one Gradle lock file."""

    packages: dict[Package, frozenset[str]]
    empty: frozenset[str]


def is_gradle_lock(path: str | Path) -> bool:
    """Whether the file name is the one of a Gradle lock file (`*.lockfile`)."""
    return Path(path).name.endswith(_SUFFIX)


def scan_arg(path: str) -> str:
    """The `-L` argument of OSV-Scanner for a lock file; a Gradle variant names its format."""
    if is_gradle_lock(path) and Path(path).name not in (GRADLE_LOCK, BUILDSCRIPT_LOCK):
        return f"{GRADLE_LOCK}:{path}"
    return path


def ships(configuration: str) -> bool:
    """Whether the packages of a configuration become part of a released build."""
    return configuration == _DESUGARING or bool(_SHIPS.match(configuration))


def parse(text: str, where: str = "lock file") -> Lock:
    """The packages of a Gradle lock file with their configurations.

    Raises:
        LockError: a line is neither a comment, the `empty=` line nor
            `group:artifact:version=configurations`.
    """
    packages: dict[Package, frozenset[str]] = {}
    empty: frozenset[str] = frozenset()
    for number, raw in enumerate(text.splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        coordinates, sep, configurations = line.partition("=")
        names = frozenset(c for c in configurations.split(",") if c)
        if sep and coordinates == "empty":
            empty = names
            continue
        parts = coordinates.split(":")
        if not sep or len(parts) != 3 or not all(parts):
            raise LockError(f"{where}:{number}: no lock entry: {line[:80]}")
        key = (f"{parts[0]}:{parts[1]}", parts[2])
        packages[key] = packages.get(key, frozenset()) | names
    return Lock(packages, empty)


def shipped(path: Path) -> set[Package]:
    """The packages of the lock file `path` that ship.

    Raises:
        LockError: the file cannot be read, or it is no build script lock file and names no
            configuration that ships.
    """
    try:
        lock = parse(path.read_text(encoding="utf-8"), str(path))
    except OSError as err:
        raise LockError(f"{path}: {err}") from err
    if path.name == BUILDSCRIPT_LOCK:
        return set()
    names = set(lock.empty).union(*lock.packages.values())
    if lock.packages and not any(ships(name) for name in names):
        raise LockError(
            f"{path} names no configuration that ships (a release runtime classpath); "
            "tools/ci/gradle_locks.py must know the layout of this build"
        )
    return {key for key, names in lock.packages.items() if any(ships(n) for n in names)}


def shipped_only(report: dict[str, Any]) -> tuple[dict[str, Any], list[tuple[str, dict[str, Any]]]]:
    """An OSV-Scanner report without the packages of Gradle lock files that do not ship.

    Returns the report with the remaining packages and the dropped ones, each with the path of
    its lock file. Results of other lock files stay as they are.

    Raises:
        LockError: a Gradle lock file of the report cannot be judged (see `shipped`).
    """
    results: list[dict[str, Any]] = []
    dropped: list[tuple[str, dict[str, Any]]] = []
    for result in report.get("results") or []:
        src = (result.get("source") or {}).get("path", "")
        if not is_gradle_lock(src):
            results.append(result)
            continue
        keep = shipped(Path(src))
        packages = []
        for pkg in result.get("packages") or []:
            p = pkg.get("package") or {}
            if (p.get("name", ""), p.get("version", "")) in keep:
                packages.append(pkg)
            else:
                dropped.append((src, pkg))
        results.append({**result, "packages": packages})
    return {**report, "results": results}, dropped
