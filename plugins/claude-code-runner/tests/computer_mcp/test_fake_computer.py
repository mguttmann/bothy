# SPDX-License-Identifier: Apache-2.0
"""Fake-Claude's MCP client of computer use against the real ``bothy-computer-mcp`` (CU-06).

The scripted agent of the end-to-end test CU-T24 (docs/spec/27-computer-use.md 15) is Fake-Claude
with the scenario key ``computer`` (``tests/fake_claude/fake_computer.py``): it reads the per-turn
MCP file of ``--mcp-config``, starts the one stdio server with that command and environment and
calls the tools of its script. This test runs one turn of the wrapper with a per-turn file of the
form of 27 10.1 and the binary built from this tree; the test plays the runner on the request
folder (27 10.2): ``list`` answers one desktop, ``open`` the refusal ``computer_not_enabled``. No
server, host or model runs. Without the Go tool chain the test fails, it never skips; it runs with
``make computer-e2e``.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import Any

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import helper

pytestmark = [pytest.mark.e2e]

ROOT = Path(__file__).resolve().parents[4]
HOST_ID = "0192f3a4-5b6c-7d8e-9f01-0000000000a1"
NONCE = "nonce-of-the-run-of-alex"
DESKTOP = "Desk of Alex"
# A bound for the whole turn of the wrapper (a test constant).
TURN_WAIT = 120.0


def _build(out: Path) -> Path:
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("the test needs the Go tool chain (go not on PATH)")
    binary = out / "bothy-computer-mcp"
    subprocess.run(
        [go, "build", "-trimpath", "-o", str(binary), "./cmd/bothy-computer-mcp"],
        cwd=ROOT / "services" / "host",
        check=True,
        env={**os.environ, "CGO_ENABLED": "0", "GOTOOLCHAIN": "local"},
    )
    return binary


def _serve(folder: Path, stop: threading.Event, seen: list[dict[str, Any]]) -> None:
    """Answer the request folder like the runner of 27 10.2 until stop is set."""
    while not stop.is_set():
        for request in sorted(folder.glob("*.request.json")):
            answer = folder / request.name.replace(".request.json", ".answer.json")
            if answer.exists():
                continue
            body = json.loads(request.read_text())
            seen.append(body)
            out: dict[str, Any] = {
                "v": 1,
                "ok": False,
                "status": 0,
                "code": "request_refused",
                "reason": None,
            }
            if body.get("nonce") == NONCE and body.get("op") == "list":
                out = {
                    "v": 1,
                    "ok": True,
                    "result": {
                        "items": [
                            {
                                "host_id": HOST_ID,
                                "name": DESKTOP,
                                "platform": "linux",
                                "online": True,
                                "locked": False,
                                "busy": False,
                            }
                        ]
                    },
                }
            elif body.get("nonce") == NONCE and body.get("op") == "open":
                out = {
                    "v": 1,
                    "ok": False,
                    "status": 403,
                    "code": "forbidden",
                    "reason": "computer_not_enabled",
                }
            tmp = answer.with_name(answer.name + ".tmp")
            tmp.write_text(json.dumps(out))
            tmp.rename(answer)
        time.sleep(0.01)


def test_fake_claude_calls_the_tools_of_its_script(tmp_path: Path) -> None:
    binary = _build(tmp_path)
    requests = tmp_path / "requests"
    requests.mkdir(mode=0o700)
    mcp_file = tmp_path / "run.computer.mcp.json"
    mcp_file.write_text(
        json.dumps(
            {
                "mcpServers": {
                    "bothy_computer": {
                        "type": "stdio",
                        "command": str(binary),
                        "args": [],
                        "env": {
                            "BOTHY_COMPUTER_DIR": str(requests),
                            "BOTHY_COMPUTER_NONCE": NONCE,
                            "BOTHY_COMPUTER_TIMEOUT_MS": "30000",
                            "BOTHY_COMPUTER_PINS": str(tmp_path / "computer" / "pins.json"),
                            "BOTHY_COMPUTER_SERVER": "https://chat.example.com",
                        },
                    }
                }
            }
        )
    )
    out = tmp_path / "computer.jsonl"
    mark = tmp_path / "listed.reached"
    capture = tmp_path / "capture.jsonl"
    claude = helper.install(
        tmp_path / "bin",
        {
            "fixture": "plain",
            "computer": {
                "out": str(out),
                "steps": [
                    {"call": "list_desktops"},
                    {"mark": str(mark)},
                    {"call": "open_session", "args": {"desktop": DESKTOP}},
                    {"call": "screenshot"},
                ],
            },
        },
        capture,
    )
    stop = threading.Event()
    seen: list[dict[str, Any]] = []
    runner = threading.Thread(target=_serve, args=(requests, stop, seen), daemon=True)
    runner.start()
    try:
        line = json.dumps({"type": "user", "message": {"role": "user", "content": "Open my desk"}})
        turn = subprocess.run(
            [str(claude), "-p", "--permission-prompts", "none", "--mcp-config", str(mcp_file)],
            input=line + "\n",
            capture_output=True,
            text=True,
            timeout=TURN_WAIT,
            check=False,
        )
    finally:
        stop.set()
        runner.join()
    assert turn.returncode == 0, turn.stderr
    records = [json.loads(x) for x in out.read_text().splitlines()]
    kinds = [r["kind"] for r in records]
    assert kinds == ["init", "tools", "call", "mark", "call", "call", "end"]
    assert records[0]["server"] == "bothy_computer"
    tools = set(records[1]["names"])
    assert {"list_desktops", "open_session", "screenshot", "zoom", "left_click", "type"} <= tools
    assert {"batch", "done", "key"} <= tools
    listed, opened, shot = records[2], records[4], records[5]
    assert not listed["is_error"]
    assert DESKTOP in listed["text"]
    assert opened["text"].startswith("not_enabled")
    assert shot["text"].startswith("no_session")
    assert records[-1]["exit"] == 0
    assert mark.exists()
    # open_session with a name lists first to find the host ID, then opens.
    assert [s["op"] for s in seen] == ["list", "list", "open"]
    assert seen[-1]["host_id"] == HOST_ID
    # The turn recorded the server it started, with the names of its variables only.
    started = [c for c in helper.read_capture(capture) if c["kind"] == "computer_server"]
    assert started == [
        {
            **started[0],
            "command": str(binary),
            "env_keys": [
                "BOTHY_COMPUTER_DIR",
                "BOTHY_COMPUTER_NONCE",
                "BOTHY_COMPUTER_PINS",
                "BOTHY_COMPUTER_SERVER",
                "BOTHY_COMPUTER_TIMEOUT_MS",
            ],
        }
    ]
    # Without the key the turn equals the turn before: no record of a computer server.
    assert '"result"' in turn.stdout


def test_without_mcp_config_the_script_records_the_refusal(tmp_path: Path) -> None:
    out = tmp_path / "computer.jsonl"
    capture = tmp_path / "capture.jsonl"
    claude = helper.install(
        tmp_path / "bin",
        {"fixture": "plain", "computer": {"out": str(out), "steps": [{"call": "list_desktops"}]}},
        capture,
    )
    line = json.dumps({"type": "user", "message": {"role": "user", "content": "Open my desk"}})
    turn = subprocess.run(
        [str(claude), "-p", "--permission-prompts", "none"],
        input=line + "\n",
        capture_output=True,
        text=True,
        timeout=TURN_WAIT,
        check=False,
    )
    assert turn.returncode == 0, turn.stderr
    records = [json.loads(x) for x in out.read_text().splitlines()]
    assert records == [{"kind": "error", "error": "no_mcp_config"}]
