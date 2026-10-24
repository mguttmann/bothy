# SPDX-License-Identifier: Apache-2.0
"""The process: `python -m cost_reconciler`, its exit codes and SIGTERM."""

from __future__ import annotations

import json
import signal
import subprocess
import sys
from pathlib import Path
from typing import Any

import pytest
from conftest import World

from cost_reconciler import __main__ as entry

ROOT = Path(__file__).resolve().parents[1]


def test_main_installs_signal_handlers(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    installed: dict[int, Any] = {}
    monkeypatch.setattr(signal, "signal", installed.__setitem__)
    code = entry.main(["--config", str(tmp_path / "absent.toml"), "--once"])
    assert code == 2
    assert set(installed) == {signal.SIGTERM, signal.SIGINT}


def test_missing_arguments_exit_2(capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as info:
        entry.main([])
    assert info.value.code == 2
    assert "--config" in capsys.readouterr().err


def test_module_runs_once_as_a_process(tmp_path: Path) -> None:
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "cost_reconciler",
            "--config",
            str(tmp_path / "absent.toml"),
            "--once",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert result.returncode == 2
    line = json.loads(result.stderr.splitlines()[-1])
    assert line["event"] == "config_error"
    assert line["key"] == "--config"
    assert result.stdout == ""


def test_sigterm_ends_the_loop(world: World) -> None:
    # One page only: the pass needs no wait, so the process waits the interval between passes.
    world.cost_api.pages = {None: world.cost_api.pages[None] | {"has_more": False}}
    config = world.write()
    process = subprocess.Popen(
        [sys.executable, "-m", "cost_reconciler", "--config", str(config)],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        assert process.stderr is not None
        events = []
        for raw in process.stderr:
            events.append(json.loads(raw)["event"])
            if events[-1] in {"pass_finished", "pass_failed"}:
                break
        assert events[-1] == "pass_finished"
        process.send_signal(signal.SIGTERM)
        stdout, _ = process.communicate(timeout=60)
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate()
    assert process.returncode == 0
    assert stdout == ""
    assert len(world.server.submissions()) == 1
    assert len(world.cost_api.requests) == 1
    for request in world.cost_api.requests:
        assert request.headers["x-api-key"] == world.admin_key
