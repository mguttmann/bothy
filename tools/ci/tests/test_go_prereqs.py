# SPDX-License-Identifier: Apache-2.0
"""Tests for go_prereqs: a server gate fails closed without go, a Docker runtime or a pin.

The PATH lookup, the process runner and the pinned-binary fetcher are injected, so the
tests need neither go nor docker nor the network."""

from __future__ import annotations

import http.client
import json
import subprocess
import urllib.error
from pathlib import Path
from typing import Any

import fetch_tools
import go_prereqs as gp
import pytest

GO = (
    "arch: go is not on PATH; install the version named in services/server/go.mod (CONTRIBUTING.md)"
)
DOCKER = (
    'arch: no Docker-compatible runtime is reachable ("docker info" failed); '
    "integration and load tests need one (docs/PLAN.md P-09)"
)


def _tool(name: str) -> dict[str, Any]:
    return {
        "version": "1.0.0",
        "repo": f"example/{name}",
        "tag": "v{version}",
        "license": "MIT",
        "binary": name,
        "assets": {
            plat: {"asset": f"{name}.tar.gz", "member": name, "sha256": "0" * 64}
            for plat in fetch_tools.PLATFORMS
        },
    }


@pytest.fixture
def lock(tmp_path: Path) -> Path:
    path = tmp_path / "tools.lock.json"
    path.write_text(json.dumps({"schema": 1, "tools": {n: _tool(n) for n in gp.TOOLS}}))
    return path


def has_all(name: str) -> str | None:
    return f"/opt/example/bin/{name}"


class Docker:
    """Fake process runner: records calls, answers `docker info` with a fixed outcome."""

    def __init__(self, returncode: int = 0, error: Exception | None = None) -> None:
        self.returncode = returncode
        self.error = error
        self.calls: list[tuple[list[str], dict[str, Any]]] = []

    def __call__(self, args: list[str], **kwargs: Any) -> subprocess.CompletedProcess[bytes]:
        self.calls.append((args, kwargs))
        if self.error is not None:
            raise self.error
        return subprocess.CompletedProcess(args, self.returncode)


class Fetcher:
    """Fake `fetch_tools.ensure`: records the tools it was asked for, can fail per tool."""

    def __init__(self, errors: dict[str, Exception] | None = None) -> None:
        self.errors = errors or {}
        self.asked: list[str] = []

    def __call__(self, lock: fetch_tools.Lock, name: str, plat: str, cache: Path) -> Path:
        self.asked.append(name)
        if name in self.errors:
            raise self.errors[name]
        return cache / name


def check(lock_path: Path, **overrides: Any) -> list[str]:
    kwargs: dict[str, Any] = {"which": has_all, "run": Docker(), "ensure": Fetcher()}
    kwargs.update(overrides)
    return gp.check("arch", lock_path=lock_path, **kwargs)


def test_all_present_passes(lock: Path) -> None:
    docker, fetcher = Docker(), Fetcher()
    assert check(lock, run=docker, ensure=fetcher) == []
    assert fetcher.asked == ["go-arch-lint", "sqlc"]
    [(args, kwargs)] = docker.calls
    assert args == ["docker", "info"]
    assert kwargs["stdout"] == subprocess.DEVNULL
    assert kwargs["stderr"] == subprocess.DEVNULL
    assert kwargs["timeout"] == gp.DOCKER_TIMEOUT_S


def test_missing_go_fails(lock: Path) -> None:
    def no_go(name: str) -> str | None:
        return None if name == "go" else has_all(name)

    assert check(lock, which=no_go) == [GO]


def test_docker_unreachable_fails(lock: Path) -> None:
    assert check(lock, run=Docker(returncode=1)) == [DOCKER]


def test_docker_binary_missing_fails(lock: Path) -> None:
    assert check(lock, run=Docker(error=FileNotFoundError("docker"))) == [DOCKER]


def test_docker_timeout_fails(lock: Path) -> None:
    timeout = subprocess.TimeoutExpired(["docker", "info"], gp.DOCKER_TIMEOUT_S)
    assert check(lock, run=Docker(error=timeout)) == [DOCKER]


def test_tool_not_pinned_fails(tmp_path: Path) -> None:
    path = tmp_path / "tools.lock.json"
    path.write_text(json.dumps({"schema": 1, "tools": {"go-arch-lint": _tool("go-arch-lint")}}))
    fetcher = Fetcher()
    assert check(path, ensure=fetcher) == ["arch: sqlc is not pinned in tools/bin/tools.lock.json"]
    assert fetcher.asked == ["go-arch-lint"]


def test_platform_without_pin_fails(tmp_path: Path) -> None:
    tool = _tool("sqlc")
    del tool["assets"][fetch_tools.platform_key()]
    path = tmp_path / "tools.lock.json"
    path.write_text(
        json.dumps({"schema": 1, "tools": {"go-arch-lint": _tool("go-arch-lint"), "sqlc": tool}})
    )
    fetcher = Fetcher()
    assert check(path, ensure=fetcher) == [
        f"arch: sqlc has no pin for {fetch_tools.platform_key()} in tools/bin/tools.lock.json"
    ]
    assert fetcher.asked == ["go-arch-lint"]


def test_fetch_error_is_reported(lock: Path) -> None:
    fetcher = Fetcher({"sqlc": fetch_tools.FetchError("sqlc: sha256 mismatch for linux-amd64")})
    assert check(lock, ensure=fetcher) == ["arch: sqlc: sqlc: sha256 mismatch for linux-amd64"]


def test_network_error_is_reported(lock: Path) -> None:
    fetcher = Fetcher({"go-arch-lint": urllib.error.URLError("no route to host")})
    assert check(lock, ensure=fetcher) == ["arch: go-arch-lint: <urlopen error no route to host>"]


def test_truncated_download_is_reported_with_the_other_problems(lock: Path) -> None:
    fetcher = Fetcher({"sqlc": http.client.IncompleteRead(b"x" * 10, 990)})
    problems = check(lock, which=lambda name: None, ensure=fetcher)
    assert problems[0] == GO
    assert problems[1] == "arch: sqlc: IncompleteRead(10 bytes read, 990 more expected)"


@pytest.mark.parametrize(
    "content",
    [
        [],
        {"schema": 1, "tools": {"sqlc": {**_tool("sqlc"), "assets": []}}},
        {"schema": 1, "tools": {"sqlc": {**_tool("sqlc"), "assets": {"linux-amd64": "x"}}}},
    ],
)
def test_malformed_lock_shape_is_reported(tmp_path: Path, content: object) -> None:
    path = tmp_path / "tools.lock.json"
    path.write_text(json.dumps(content))
    problems = check(path, which=lambda name: None)
    assert problems[0] == GO
    assert [p.split(": ")[1] for p in problems[1:]] == ["go-arch-lint", "sqlc"]


def test_unreadable_lock_is_reported(tmp_path: Path) -> None:
    path = tmp_path / "tools.lock.json"
    path.write_text(json.dumps({"schema": 2, "tools": {}}))
    problems = check(path)
    assert [p.split(": ")[1] for p in problems] == ["go-arch-lint", "sqlc"]
    assert all("expected schema 1" in p for p in problems)
    assert check(tmp_path / "absent.json")[0].startswith("arch: go-arch-lint: ")


def test_all_problems_reported_together(tmp_path: Path) -> None:
    path = tmp_path / "tools.lock.json"
    path.write_text(json.dumps({"schema": 1, "tools": {"go-arch-lint": _tool("go-arch-lint")}}))
    problems = check(path, which=lambda name: None, run=Docker(returncode=1))
    assert problems == [GO, DOCKER, "arch: sqlc is not pinned in tools/bin/tools.lock.json"]


def test_main_exit_codes(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    gates: list[str] = []

    def clean(gate: str) -> list[str]:
        gates.append(gate)
        return []

    monkeypatch.setattr(gp, "check", clean)
    assert gp.main(["perf"]) == 0
    assert gates == ["perf"]
    assert capsys.readouterr() == ("", "")

    monkeypatch.setattr(gp, "check", lambda gate: [f"{gate}: first", f"{gate}: second"])
    assert gp.main(["test"]) == 1
    assert capsys.readouterr().err == "test: first\ntest: second\n"

    assert gp.main([]) == 2
