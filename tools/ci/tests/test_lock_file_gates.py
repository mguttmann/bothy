# SPDX-License-Identifier: Apache-2.0
"""Every tracked lock file is read by the advisory gate, the Gradle ones also by the license gate.

`make osv` takes its lock files from two lists of the Makefile (`LOCK_NAMES`, `GRADLE_LOCKS`).
A lock file under a name neither list knows would be scanned by nothing, as the Gradle lock
files of the Android builds were (docs/decisions.d/AUD-root-01.md). The tests therefore list the
tracked lock files by the naming conventions of the package managers, run the real recipes of
the Makefile with a recording scanner and compare. One test runs the pinned OSV-Scanner itself,
offline against a generated advisory database, over the tracked Gradle lock files: a vulnerable
package that ships fails the gate, one that only the build or its tests resolve does not.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import zipfile
from pathlib import Path

import advisories
import pytest

REPO = Path(__file__).resolve().parents[3]

# Lock files by the naming conventions of the package managers (uv.lock, pubspec.lock,
# Cargo.lock, package-lock.json, pnpm-lock.yaml, gradle.lockfile, Package.resolved, go.mod).
LOCK_FILE = re.compile(
    r"(^|/)(go\.mod|Package\.resolved|[^/]*\.lock|[^/]*\.lockfile"
    r"|[^/]*[-.]lock\.(json|ya?ml|hcl|toml))$"
)
# Lock files that hold no packages of a package registry, by file name, each with its reason.
NO_PACKAGE_LOCK = {
    ".terraform.lock.hcl": "OpenTofu provider pins with their hashes; `make tofu-check` checks "
    "every archive against them, and OSV-Scanner has no parser for the file",
    "tools.lock.json": "pins of release binaries by URL and sha256 (tools/bin/fetch_tools.py)",
}
# File names OSV-Scanner v2 picks a parser for; any other name must name its format.
SCANNER_NAMES = {
    "uv.lock",
    "package-lock.json",
    "pnpm-lock.yaml",
    "pubspec.lock",
    "go.mod",
    "gradle.lockfile",
    "buildscript-gradle.lockfile",
}
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "OSV_BASE"}

_FAKE_SCANNER = """#!{python}
import json, os, sys
with open(os.environ["FAKE_OSV_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps(sys.argv[1:]) + "\\n")
print("{{}}")
"""


def _tracked(*patterns: str) -> list[str]:
    out = subprocess.run(
        ["git", "ls-files", "--", *patterns], cwd=REPO, check=True, capture_output=True, text=True
    ).stdout
    return sorted(out.split())


def tracked_lock_files() -> list[str]:
    """The tracked lock files of package managers, found by name alone."""
    return [
        path
        for path in _tracked()
        if LOCK_FILE.search(path) and Path(path).name not in NO_PACKAGE_LOCK
    ]


class Recorder:
    """A fake OSV-Scanner that records its arguments, reached through `FETCH`, and a fake `go`."""

    def __init__(self, tmp: Path) -> None:
        self.log = tmp / "osv-calls.jsonl"
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        self.scanner = bin_dir / "osv-scanner"
        self.scanner.write_text(_FAKE_SCANNER.format(python=sys.executable), encoding="utf-8")
        self.fetch = bin_dir / "fetch"
        self.fetch.write_text(f"#!/bin/sh\necho {self.scanner}\n", encoding="utf-8")
        go = bin_dir / "go"
        go.write_text(
            '#!/bin/sh\necho \'{"config": {"scanner_name": "govulncheck"}}\'\n', encoding="utf-8"
        )
        for exe in (self.scanner, self.fetch, go):
            exe.chmod(0o755)
        self.env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
        self.env["PATH"] = f"{bin_dir}{os.pathsep}{self.env['PATH']}"
        self.env["FAKE_OSV_LOG"] = str(self.log)

    def make(self, *args: str, **env: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["make", "-s", "-C", str(REPO), *args, f"FETCH={self.fetch}", "COMPONENTS=all"],
            env={**self.env, **env},
            capture_output=True,
            text=True,
            check=False,
            timeout=600,
        )

    def lock_arguments(self) -> list[str]:
        """Every `-L` argument of every scanner call."""
        if not self.log.exists():
            return []
        calls = [json.loads(line) for line in self.log.read_text(encoding="utf-8").splitlines()]
        return [call[i + 1] for call in calls for i, arg in enumerate(call) if arg == "-L"]


def _files(arguments: list[str]) -> set[str]:
    """The lock files behind scanner arguments; a name the scanner does not know names a format."""
    files = set()
    for argument in arguments:
        form, sep, path = argument.partition(":")
        if not sep:
            form, path = Path(argument).name, argument
        assert form in SCANNER_NAMES, f"OSV-Scanner picks no parser for {argument}"
        files.add(path)
    return files


def test_the_repository_tracks_lock_files_of_every_kind() -> None:
    names = {Path(p).name for p in tracked_lock_files()}
    assert {"uv.lock", "package-lock.json", "pubspec.lock", "go.mod", "gradle.lockfile"} <= names
    assert "gradle-firebase.lockfile" in names, "the variant lock file of the Firebase build"


@pytest.mark.parametrize("base", [None, "HEAD"], ids=["full-scan", "pull-request"])
def test_every_tracked_lock_file_reaches_the_advisory_scan(
    tmp_path: Path, base: str | None
) -> None:
    """A lock file under a name the Makefile does not list is scanned by nothing."""
    recorder = Recorder(tmp_path)
    result = recorder.make("osv", **({"OSV_BASE": base} if base else {}))
    assert result.returncode == 0, result.stdout + result.stderr
    assert _files(recorder.lock_arguments()) == set(tracked_lock_files())


def _recipe_command(recorder: Recorder, target: str, marker: str) -> str:
    """The shell command of `target` that contains `marker`, as `make -n` prints it."""
    printed = recorder.make("-n", target).stdout
    commands: list[str] = []
    for line in printed.splitlines():
        if commands and (commands[-1].endswith("\\") or line.startswith("\t")):
            commands[-1] += "\n" + line
        else:
            commands.append(line)
    [command] = [c for c in commands if marker in c]
    return command


def test_the_gradle_lock_files_of_the_apps_reach_the_license_scan(tmp_path: Path) -> None:
    """The Maven packages of the Android builds ship, so the license list applies to them."""
    recorder = Recorder(tmp_path)
    command = _recipe_command(recorder, "licenses", "dep_licenses.py --osv-scanner")
    result = subprocess.run(
        ["bash", "-euo", "pipefail", "-c", command],
        cwd=REPO,
        env=recorder.env,
        capture_output=True,
        text=True,
        check=False,
        timeout=600,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    scanned = _files(recorder.lock_arguments())
    gradle = set(_tracked("apps/*.lockfile", "packages/*.lockfile"))
    assert gradle, "no Gradle lock file under apps/ or packages/"
    assert gradle <= scanned
    assert "pubspec.lock" in scanned


# The pinned scanner itself, offline.


def _pinned_scanner() -> Path:
    bin_dir = str(REPO / "tools" / "bin")
    if bin_dir not in sys.path:
        sys.path.insert(0, bin_dir)
    import fetch_tools

    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return fetch_tools.ensure(
        lock, "osv-scanner", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE
    )


def _entries(lock: str) -> list[tuple[str, str, set[str]]]:
    """(group:artifact, version, configurations) of a lock file, read without the tool."""
    entries = []
    for line in (REPO / lock).read_text(encoding="utf-8").splitlines():
        if line.startswith(("#", "empty=")) or "=" not in line:
            continue
        coordinates, configurations = line.split("=", 1)
        group, artifact, version = coordinates.split(":")
        entries.append((f"{group}:{artifact}", version, set(configurations.split(","))))
    return entries


def _offline_scanner(tmp: Path, vulnerable: dict[tuple[str, str], str]) -> Path:
    """The pinned scanner behind a wrapper that makes it read a local advisory database only."""
    maven = tmp / "db" / "osv-scalibr" / "Maven"
    maven.mkdir(parents=True)
    with zipfile.ZipFile(maven / "all.zip", "w") as archive:
        for (name, version), advisory in vulnerable.items():
            record = {
                "schema_version": "1.6.0",
                "id": advisory,
                "modified": "2000-01-01T00:00:00Z",
                "published": "2000-01-01T00:00:00Z",
                "summary": "made up for the test",
                "affected": [
                    {"package": {"ecosystem": "Maven", "name": name}, "versions": [version]}
                ],
            }
            archive.writestr(f"{advisory}.json", json.dumps(record))
    wrapper = tmp / "osv-scanner-offline"
    wrapper.write_text(
        "#!/bin/sh\n"
        "shift 2\n"
        f'exec "{_pinned_scanner()}" scan source --offline --local-db-path "{tmp / "db"}" "$@"\n',
        encoding="utf-8",
    )
    wrapper.chmod(0o755)
    return wrapper


def test_the_pinned_scanner_fails_the_gate_for_a_vulnerable_package_that_ships(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    """The full scan of `make osv` over the tracked Gradle lock files, with the real scanner.

    One package that ships and one that only the build or its tests resolve get a made-up
    advisory. The scanner reads every lock file under the argument the gate passes (a variant
    such as gradle-firebase.lockfile only with its format named), and the gate fails for the
    first package in every lock file that ships it and for nothing else.
    """
    locks = _tracked("*.lockfile")
    ships = "releaseRuntimeClasspath"
    app = _entries("apps/client/android/gradle.lockfile")
    shipped = next((n, v) for n, v, confs in app if ships in confs)
    build_only = next((n, v) for n, v, confs in app if not confs & {ships, "coreLibraryDesugaring"})
    advisory = {shipped: "OSV-2000-1", build_only: "OSV-2000-2"}
    scanner = _offline_scanner(tmp_path, advisory)

    argv = ["--repo", str(REPO), "--osv-scanner", str(scanner)]
    for lock in locks:
        argv += ["--lock", lock]
    assert advisories.main(argv) == 1
    out = capsys.readouterr().out

    errors = {line for line in out.splitlines() if line.startswith("::error::")}
    expected = set()
    outside: dict[str, int] = {}
    for lock in locks:
        for name, version, confs in _entries(lock):
            if (name, version) not in advisory:
                continue
            if confs & {ships, "coreLibraryDesugaring"}:
                expected.add(
                    f"::error::vulnerable dependency: {lock}: Maven {name} {version}: "
                    f"{advisory[(name, version)]}"
                )
            else:
                outside[lock] = outside.get(lock, 0) + 1
    assert expected, "no lock file ships the package"
    assert errors == expected
    assert outside
    for lock, count in outside.items():
        assert (
            f"advisories: {lock}: {count} package(s) with a known vulnerability do not ship" in out
        )
