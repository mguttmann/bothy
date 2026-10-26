# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/ci/terminal_stack.py, the stack test of the app's terminal (brief M4-36)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import terminal_stack


class _Helper:
    """A stand-in for the Go helper: writes the connection file or ends early."""

    def __init__(self, env: dict[str, str], write: bool, rc: int) -> None:
        self.out = Path(env["BOTHY_CLIENTSTACK_OUT"])
        self.write = write
        self.rc = rc
        if write:
            self.out.write_text(json.dumps({"origin": "https://chat.example.com"}))

    def poll(self) -> int | None:
        return None if self.write else self.rc

    def wait(self) -> int:
        return self.rc


def _run(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    *,
    write: bool,
    helper_rc: int,
    dart_rc: int,
    mode: str | None = None,
) -> tuple[int, dict[str, object]]:
    seen: dict[str, object] = {}

    def popen(args: list[str], cwd: Path, env: dict[str, str]) -> _Helper:
        seen["helper"] = args
        seen["out"] = Path(env["BOTHY_CLIENTSTACK_OUT"])
        seen["helper_mode"] = env["BOTHY_CLIENTSTACK_MODE"]
        return _Helper(env, write, helper_rc)

    def call(args: list[str], cwd: Path, env: dict[str, str]) -> int:
        seen["dart"] = args
        seen["dart_cwd"] = cwd
        seen["terminal_stack"] = env["TERMINAL_STACK"]
        return dart_rc

    monkeypatch.setattr(terminal_stack.subprocess, "Popen", popen)
    monkeypatch.setattr(terminal_stack.subprocess, "call", call)
    monkeypatch.setattr(terminal_stack, "POLL_SECONDS", 0)
    monkeypatch.setattr(terminal_stack.tempfile, "tempdir", str(tmp_path))
    argv = ["--root", str(tmp_path)] + (["--mode", mode] if mode else [])
    return terminal_stack.main(argv), seen


def test_passes_when_test_and_helper_pass(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    rc, seen = _run(monkeypatch, tmp_path, write=True, helper_rc=0, dart_rc=0)
    assert rc == 0
    assert "^TestClientStack$" in seen["helper"]
    assert seen["dart"] == ["flutter", "test", "terminal_stack_test/terminal_stack_test.dart"]
    # The work folder with the connection file and the stop file is gone.
    assert not Path(seen["out"]).parent.exists()


def test_fails_when_the_helper_ends_before_the_stack_is_ready(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    rc, seen = _run(monkeypatch, tmp_path, write=False, helper_rc=1, dart_rc=0)
    assert rc == 1
    assert "dart" not in seen
    assert not Path(seen["out"]).parent.exists()


def test_a_failed_removal_of_the_stack_fails_a_passing_test(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    rc, _ = _run(monkeypatch, tmp_path, write=True, helper_rc=1, dart_rc=0)
    assert rc == 1


def test_a_failed_dart_test_fails(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    rc, _ = _run(monkeypatch, tmp_path, write=True, helper_rc=0, dart_rc=1)
    assert rc == 1


def test_the_stop_file_is_written_on_every_path(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    stops: list[str] = []
    real_touch = Path.touch

    def touch(self: Path, *args: object, **kwargs: object) -> None:
        stops.append(self.name)
        real_touch(self, *args, **kwargs)

    monkeypatch.setattr(Path, "touch", touch)
    for write in (True, False):
        _run(monkeypatch, tmp_path, write=write, helper_rc=1, dart_rc=1)
    assert stops == ["connection.json.stop", "connection.json.stop"]


@pytest.mark.parametrize(
    ("mode", "helper_mode", "command"),
    [
        (None, "", ["flutter", "test", "terminal_stack_test/terminal_stack_test.dart"]),
        ("e2e", "e2e", ["tool/e2e_suite.sh", "terminal"]),
        ("perf", "perf", ["tool/e2e_suite.sh", "terminal-perf"]),
    ],
)
def test_the_mode_selects_the_helper_and_the_test(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    mode: str | None,
    helper_mode: str,
    command: list[str],
) -> None:
    rc, seen = _run(monkeypatch, tmp_path, write=True, helper_rc=0, dart_rc=0, mode=mode)
    assert rc == 0
    assert seen["helper_mode"] == helper_mode
    assert seen["dart"] == command
    assert Path(str(seen["dart_cwd"])).name == "client"
    # The suite runner reads the connection file the helper wrote.
    assert seen["terminal_stack"] == str(seen["out"])


def test_an_unknown_mode_is_refused(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    with pytest.raises(SystemExit):
        _run(monkeypatch, tmp_path, write=True, helper_rc=0, dart_rc=0, mode="other")
