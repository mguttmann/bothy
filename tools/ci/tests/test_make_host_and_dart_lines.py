# SPDX-License-Identifier: Apache-2.0
"""The lines of the root Makefile that the host module and the first Dart package exercise
(brief M2-38, Design note 3; docs/DECISIONS.md D-M2-38-3 and D-M2-38-4).

Each test runs a copy of the root Makefile in a temporary git repository: the component
discovery reads tracked files only, so the repository tracks exactly the manifests of the case.
`tools/` is empty, and fake `uv`, `flutter` and `dart` executables come first on PATH, so the
Python tool calls of the recipes succeed and the Dart tool calls are recorded. No network, no
Go and no Flutter are needed.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "COVERAGE_DART_MIN", "GO_MODS"}

_FAKE = """#!{python}
import json, os, sys
name = os.path.basename(sys.argv[0])
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"tool": name, "args": sys.argv[1:], "cwd": os.getcwd()}}) + "\\n")
if name == "flutter" and sys.argv[1:2] == ["pub"] and os.environ.get("FAKE_FLUTTER_PUB_EXIT"):
    sys.exit(int(os.environ["FAKE_FLUTTER_PUB_EXIT"]))
if name == "flutter" and sys.argv[1:2] == ["test"] and os.environ.get("FAKE_LCOV"):
    os.makedirs("coverage", exist_ok=True)
    with open("coverage/lcov.info", "w", encoding="utf-8") as fh:
        fh.write(os.environ["FAKE_LCOV"])
sys.exit(0)
"""


class Repo:
    """A temporary repository with a copy of the root Makefile and recording fakes."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.bin = root.parent / "bin"
        self.log = root.parent / "calls.jsonl"
        root.mkdir()
        shutil.copy(REPO / "Makefile", root / "Makefile")
        (root / "tools").mkdir()
        fake = _FAKE.format(python=sys.executable)
        for name in ("uv", "flutter", "dart"):
            path = self.bin / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(fake, encoding="utf-8")
            path.chmod(0o755)

    def write(self, rel: str, text: str = "") -> Path:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def commit(self) -> None:
        for args in (
            ["init", "-q", "-b", "main"],
            ["config", "core.hooksPath", "/dev/null"],
            ["add", "."],
            ["-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "x"],
        ):
            subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

    def environ(self, **env: str) -> dict[str, str]:
        environ = {k: v for k, v in os.environ.items() if k not in _DROPPED}
        environ["PATH"] = f"{self.bin}{os.pathsep}{environ['PATH']}"
        environ.update(FAKE_LOG=str(self.log), **env)
        return environ

    def make(self, *args: str, **env: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["make", "-s", "-C", str(self.root), *args],
            env=self.environ(**env),
            capture_output=True,
            text=True,
            check=False,
            timeout=120,
        )

    def calls(self, tool: str) -> list[dict[str, object]]:
        if not self.log.exists():
            return []
        entries = [json.loads(line) for line in self.log.read_text().splitlines()]
        return [e for e in entries if e["tool"] == tool]


@pytest.fixture
def repo(tmp_path: Path) -> Repo:
    return Repo(tmp_path / "repo")


def test_perf_skips_go_module_without_performance_targets(repo: Repo) -> None:
    repo.write("services/host/go.mod", "module example.com/host\n")
    repo.commit()
    result = repo.make("perf")
    assert result.returncode == 0, result.stdout + result.stderr
    assert "SKIP  perf" in result.stdout
    assert "no Go module with performance targets selected" in result.stdout
    # The prerequisite check (uv) does not run when nothing is to be measured.
    assert repo.calls("uv") == []


def test_perf_still_requires_perf_dir_of_listed_module(repo: Repo) -> None:
    repo.write("services/server/go.mod", "module example.com/server\n")
    repo.write("services/host/go.mod", "module example.com/host\n")
    repo.commit()
    result = repo.make("perf")
    assert result.returncode == 2
    assert "perf: services/server/perf does not exist" in result.stderr
    assert "services/host" not in result.stderr


def _dart_package(repo: Repo, *, tests: bool = True) -> None:
    repo.write("packages/example_pkg/pubspec.yaml", "name: example_pkg\n")
    repo.write(
        "packages/example_pkg/lib/example_pkg.dart", "// SPDX-License-Identifier: Apache-2.0\n"
    )
    if tests:
        repo.write("packages/example_pkg/test/example_test.dart", "void main() {}\n")
    repo.commit()


def test_dart_package_with_tests_runs_flutter_test(repo: Repo) -> None:
    _dart_package(repo)
    result = repo.make("test", "COMPONENTS=apps")
    assert result.returncode == 0, result.stdout + result.stderr
    package = str((repo.root / "packages" / "example_pkg").resolve())
    calls = repo.calls("flutter")
    # A stale pubspec.lock fails before the tests instead of being rewritten silently.
    assert [c["args"] for c in calls] == [
        ["pub", "get", "--enforce-lockfile"],
        ["test", "--coverage"],
    ]
    assert {c["cwd"] for c in calls} == {package}


def test_dart_lint_enforces_the_lockfile(repo: Repo) -> None:
    _dart_package(repo)
    command = _recipe(repo, "lint", "flutter analyze")
    result = subprocess.run(
        ["bash", "-c", command],
        cwd=repo.root,
        env=repo.environ(),
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert [c["args"] for c in repo.calls("flutter")] == [
        ["pub", "get", "--enforce-lockfile"],
        ["analyze", "--fatal-infos"],
    ]
    failing = subprocess.run(
        ["bash", "-c", command],
        cwd=repo.root,
        env=repo.environ(FAKE_FLUTTER_PUB_EXIT="1"),
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )
    assert failing.returncode != 0
    assert ["analyze", "--fatal-infos"] not in [c["args"] for c in repo.calls("flutter")[2:]]


def _recipe(repo: Repo, target: str, marker: str) -> str:
    """The shell command of `target` that contains `marker`, as `make -n` prints it."""
    printed = repo.make("-n", target, "COMPONENTS=apps").stdout
    commands: list[str] = []
    for line in printed.splitlines():
        if commands and (commands[-1].endswith("\\") or line.startswith("\t")):
            commands[-1] += "\n" + line
        else:
            commands.append(line)
    [command] = [c for c in commands if marker in c]
    return command


def test_dart_package_without_tests_fails(repo: Repo) -> None:
    _dart_package(repo, tests=False)
    result = repo.make("test", "COMPONENTS=apps")
    assert result.returncode == 2
    assert (
        "packages/example_pkg: no test/ directory (docs/spec/23-qualitaet.md, 3)" in result.stderr
    )
    assert repo.calls("flutter") == []


def test_dart_coverage_below_threshold_fails(repo: Repo) -> None:
    _dart_package(repo)
    result = repo.make("coverage", "COMPONENTS=apps", FAKE_LCOV="SF:lib/a.dart\nLF:10\nLH:5\n")
    assert result.returncode == 2
    assert "packages/example_pkg: coverage 50.0% < 80%" in result.stdout + result.stderr
    passed = repo.make("coverage", "COMPONENTS=apps", FAKE_LCOV="SF:lib/a.dart\nLF:10\nLH:9\n")
    assert passed.returncode == 0, passed.stdout + passed.stderr


def test_dart_coverage_without_lcov_fails(repo: Repo) -> None:
    _dart_package(repo)
    result = repo.make("coverage", "COMPONENTS=apps")
    assert result.returncode == 2
    assert (
        "packages/example_pkg: coverage/lcov.info is missing; make test writes it" in result.stderr
    )
