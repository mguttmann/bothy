# SPDX-License-Identifier: Apache-2.0
"""Fake-Claude's MCP client of computer use: the scripted agent of CU-T24 (CU-06).

Standard library only. A turn whose scenario has the key ``computer`` runs this script after the
``system/init`` line and before its fixture, like a model that calls MCP tools before it answers.
Without the key Fake-Claude behaves exactly as before. The scenario value::

    {"out": "<file>", "steps": [<step>, ...]}

It reads the per-turn MCP file named by ``--mcp-config`` (docs/spec/27-computer-use.md 10.1),
starts its server ``bothy_computer`` with exactly the file's ``command`` and ``args`` and the
file's ``env`` on top of the turn's environment (as the CLI starts a ``stdio`` server), speaks
JSON-RPC over the server's stdin and stdout (``initialize`` of the revision ``2025-11-25``, then
``tools/list``) and runs the steps in order:

* ``{"call": <tool>, "args": {...}}``: one ``tools/call``. With ``"until_prefix": <text>`` the
  call repeats until its text starts with that text, with ``"until_ok": true`` until the result
  is no error; the last result counts. A condition that does not hold within the patience of the
  gates stops the script.
* ``{"mark": <path>}``: creates the file, so the test knows the script got there.
* ``{"gate": <path>}``: waits until the test creates the file.

Every step and its outcome is one JSON line of ``out``: ``init`` (``protocolVersion``, ``server``),
``tools`` (``names``), ``call`` (``step``, ``tool``, ``is_error``, ``text``, ``images`` as
``{"mime", "data"}`` with the base64 data of the server, ``attempts``), ``mark``, ``gate``,
``error`` (``no_mcp_config``, ``no_server``, ``unmet``) and ``end`` (``exit``, the server's exit
code after its stdin closed). The capture gets one ``computer_server`` record with the command and
the sorted names of the file's variables, never their values. The server's stderr goes to
``<out>.server.log``.

Example scenario::

    {"fixture": "plain", "computer": {"out": "/tmp/cu.jsonl",
     "steps": [{"call": "list_desktops"}, {"call": "open_session", "args": {"desktop": "Desk"}}]}}
"""

from __future__ import annotations

import json
import os
import subprocess
import time
from collections.abc import Callable
from pathlib import Path
from typing import IO, Any

SERVER = "bothy_computer"
# The revision with initialize that bothy-computer-mcp speaks (D-CU-05-1).
PROTOCOL = "2025-11-25"
# How often a repeated call or a gate looks again; a pace, not a wait the test relies on.
POLL = 0.1


class UnmetError(Exception):
    """A gate or a repeated call whose condition did not hold within the patience."""


class _Client:
    """A JSON-RPC client of one stdio server."""

    def __init__(self, proc: subprocess.Popen[bytes]) -> None:
        assert proc.stdin is not None
        assert proc.stdout is not None
        self.proc = proc
        self.stdin: IO[bytes] = proc.stdin
        self.stdout: IO[bytes] = proc.stdout
        self.next_id = 0

    def notify(self, method: str, params: dict[str, Any] | None = None) -> None:
        """Send one notification (no answer)."""
        message: dict[str, Any] = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            message["params"] = params
        self.stdin.write(json.dumps(message).encode() + b"\n")
        self.stdin.flush()

    def request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        """Send one request and return its result; an error answer raises RuntimeError."""
        self.next_id += 1
        ident = self.next_id
        line = {"jsonrpc": "2.0", "id": ident, "method": method, "params": params}
        self.stdin.write(json.dumps(line).encode() + b"\n")
        self.stdin.flush()
        while True:
            raw = self.stdout.readline()
            if not raw:
                raise RuntimeError(f"{method}: the server closed its output")
            answer = json.loads(raw)
            if answer.get("id") != ident:
                continue  # a notification of the server
            if "error" in answer:
                raise RuntimeError(f"{method}: error {answer['error'].get('code')}")
            result: dict[str, Any] = answer.get("result") or {}
            return result


def run(fake: Any, argv: list[str]) -> None:
    """Run the ``computer`` script of the scenario of ``fake`` in this turn.

    Args:
        fake: The `fake_claude.Fake` of the turn (scenario, ``record`` and the gate patience).
        argv: The turn's arguments.
    """
    spec = fake.scenario["computer"]
    out = Path(spec["out"])

    def write(entry: dict[str, Any]) -> None:
        with out.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(entry) + "\n")

    config_path = _after(argv, "--mcp-config")
    if config_path is None:
        write({"kind": "error", "error": "no_mcp_config"})
        return
    servers = json.loads(Path(config_path).read_text()).get("mcpServers", {})
    server = servers.get(SERVER)
    if not isinstance(server, dict):
        write({"kind": "error", "error": "no_server"})
        return
    command = [str(server["command"]), *[str(a) for a in server.get("args", [])]]
    env = {**os.environ, **{str(k): str(v) for k, v in server.get("env", {}).items()}}
    fake.record(
        "computer_server", command=command[0], env_keys=sorted(server.get("env", {}).keys())
    )
    with Path(f"{out}.server.log").open("ab") as log:
        proc = subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log, env=env
        )
        try:
            _script(_Client(proc), spec["steps"], write, float(fake.patience))
        except UnmetError as err:
            write({"kind": "error", "error": "unmet", "detail": str(err)})
        finally:
            if proc.stdin is not None:
                proc.stdin.close()
            try:
                code = proc.wait(timeout=float(fake.patience))
            except subprocess.TimeoutExpired:
                proc.kill()
                code = proc.wait()
            write({"kind": "end", "exit": code})


def _script(
    client: _Client,
    steps: list[dict[str, Any]],
    write: Callable[[dict[str, Any]], None],
    patience: float,
) -> None:
    """Initialize, list the tools and run the steps, writing one line per step."""
    init = client.request(
        "initialize",
        {
            "protocolVersion": PROTOCOL,
            "capabilities": {},
            "clientInfo": {"name": "fake-claude", "version": "0"},
        },
    )
    client.notify("notifications/initialized")
    write(
        {
            "kind": "init",
            "protocolVersion": init.get("protocolVersion"),
            "server": (init.get("serverInfo") or {}).get("name"),
        }
    )
    tools = client.request("tools/list", {}).get("tools", [])
    write({"kind": "tools", "names": [t.get("name") for t in tools]})
    for index, step in enumerate(steps):
        if "mark" in step:
            Path(step["mark"]).touch()
            write({"kind": "mark", "step": index, "path": step["mark"]})
        elif "gate" in step:
            _wait(lambda p=Path(step["gate"]): p.exists(), patience, f"gate {step['gate']}")
            write({"kind": "gate", "step": index, "path": step["gate"]})
        else:
            write(_call(client, index, step, patience))


def _call(client: _Client, index: int, step: dict[str, Any], patience: float) -> dict[str, Any]:
    """Run one call step, repeated while its condition does not hold."""
    deadline = time.monotonic() + patience
    attempts = 0
    while True:
        attempts += 1
        result = client.request(
            "tools/call", {"name": step["call"], "arguments": step.get("args", {})}
        )
        entry = _entry(index, step["call"], result, attempts)
        if _holds(step, entry):
            return entry
        if time.monotonic() > deadline:
            raise UnmetError(f"step {index} {step['call']}: {entry['text'][:80]}")
        time.sleep(POLL)


def _holds(step: dict[str, Any], entry: dict[str, Any]) -> bool:
    """Report whether the result of a call step meets its condition."""
    prefix = step.get("until_prefix")
    if prefix is not None and not str(entry["text"]).startswith(str(prefix)):
        return False
    return not (step.get("until_ok") and entry["is_error"])


def _entry(index: int, tool: str, result: dict[str, Any], attempts: int) -> dict[str, Any]:
    """Return the output line of one tool result."""
    content = result.get("content") or []
    texts = [c.get("text", "") for c in content if c.get("type") == "text"]
    images = [
        {"mime": c.get("mimeType"), "data": c.get("data")}
        for c in content
        if c.get("type") == "image"
    ]
    return {
        "kind": "call",
        "step": index,
        "tool": tool,
        "is_error": bool(result.get("isError")),
        "text": texts[0] if texts else "",
        "images": images,
        "attempts": attempts,
    }


def _wait(cond: Callable[[], bool], patience: float, what: str) -> None:
    """Wait until cond holds; raise UnmetError after patience seconds."""
    deadline = time.monotonic() + patience
    while not cond():
        if time.monotonic() > deadline:
            raise UnmetError(what)
        time.sleep(POLL)


def _after(argv: list[str], flag: str) -> str | None:
    """Return the argument after flag, or None."""
    if flag in argv:
        index = argv.index(flag)
        if index + 1 < len(argv):
            return argv[index + 1]
    return None
