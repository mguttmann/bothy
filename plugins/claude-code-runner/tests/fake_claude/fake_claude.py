# SPDX-License-Identifier: Apache-2.0
"""Fake-Claude: an executable that plays stream-json fixtures (09 16.1 row "Fake Claude").

Standard library only. It runs through a wrapper that `helper.install` writes with the test
interpreter as its shebang and the scenario and capture paths baked in, because the allow-list
environment of a turn drops every control variable. Commands:

* ``--version``: prints ``<version> (Claude Code)``.
* ``auth status``: ``{"loggedIn": ..., "authMethod": ...}`` with ``api_key`` when
  ``ANTHROPIC_API_KEY`` is set, else the content of ``$CLAUDE_CONFIG_DIR/fake-login`` (when
  ``CLAUDE_CONFIG_DIR`` is set) or of ``$HOME/.claude/fake-login``, else ``none``.
* ``-p``: records argv, cwd, environment, the system prompt file and every stdin line, emits
  ``system/init``, echoes the first stdin line with ``isReplay``, plays the fixture, emits the
  ``result`` and exits after stdin EOF. SIGINT ends it with ``error_during_execution``.

Fake-Claude never waits a fixed time: a scenario holds a command with a gate, a file path the
test creates when it wants the command to go on (``auth_gate`` before the answer of
``auth status``, ``turn_gate`` before the ``result`` of a turn). ``helper.install`` refuses the
former wall-clock keys, so a test cannot rest on a race between a sleep and the machine's load.

Fixtures are constructed from the message shapes of the Agent SDK reference, not recorded.
"""

from __future__ import annotations

import contextlib
import json
import os
import signal
import subprocess
import sys
import threading
import time
from collections.abc import Iterator
from pathlib import Path
from typing import Any

FIXTURES = Path(__file__).resolve().parent / "fixtures"
PINNED = "2.1.259"
# quality.test.patience (bothy_sdk.testing._patience.PATIENCE); standard library only here.
GATE_PATIENCE = 180.0


class Fake:
    def __init__(self, scenario: dict[str, Any], capture: Path) -> None:
        self.scenario = scenario
        self.capture = capture
        self.lock = threading.RLock()
        self.session_id = ""
        self.model = ""
        self.stopping = threading.Event()

    def record(self, kind: str, **data: Any) -> None:
        entry = {"kind": kind, "pid": os.getpid(), "t": time.monotonic(), **data}
        with _no_sigint(), self.lock, self.capture.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(entry) + "\n")
            fh.flush()

    def emit(self, message: dict[str, Any]) -> None:
        with _no_sigint(), self.lock:
            sys.stdout.write(json.dumps(message) + "\n")
            sys.stdout.flush()

    # ---- commands --------------------------------------------------------------------------

    def version(self) -> int:
        self.record("version", argv=sys.argv[1:])
        if "version_raw" in self.scenario:
            sys.stdout.write(str(self.scenario["version_raw"]))
        else:
            print(f"{self.scenario.get('version', PINNED)} (Claude Code)")
        sys.stdout.flush()
        return int(self.scenario.get("version_exit", 0))

    def auth_status(self) -> int:
        env = dict(os.environ)
        self.record("auth_status", argv=sys.argv[1:], env=env, cwd=os.getcwd())
        self.gate("auth_gate")
        if "auth_raw" in self.scenario:
            sys.stdout.write(str(self.scenario["auth_raw"]))
            return int(self.scenario.get("auth_exit", 0))
        method = self.scenario.get("auth_method")
        if method is None:
            method = "none"
            if env.get("ANTHROPIC_API_KEY"):
                method = "api_key"
            else:
                base = env.get("CLAUDE_CONFIG_DIR")
                login = (
                    Path(base) / "fake-login"
                    if base
                    else Path(env.get("HOME", "/nonexistent")) / ".claude" / "fake-login"
                )
                if login.is_file():
                    method = login.read_text().strip() or "none"
        print(json.dumps({"loggedIn": method != "none", "authMethod": method}))
        return int(self.scenario.get("auth_exit", 1 if method == "none" else 0))

    def turn(self) -> int:
        argv = sys.argv[1:]
        prompt_file = _after(argv, "--append-system-prompt-file")
        system_prompt = (
            Path(prompt_file).read_text() if prompt_file and Path(prompt_file).is_file() else None
        )
        self.session_id = _after(argv, "--session-id") or _after(argv, "--resume") or ""
        self.model = _after(argv, "--model") or ""
        self.record(
            "turn_start",
            program=sys.argv[0],
            argv=argv,
            cwd=os.getcwd(),
            env=dict(os.environ),
            system_prompt=system_prompt,
        )
        signal.signal(
            signal.SIGINT, signal.SIG_IGN if self.scenario.get("ignore_sigint") else self.on_sigint
        )
        if self.scenario.get("ignore_sigterm"):
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
        if self.scenario.get("spawn_child"):
            # A background job of a tool call: it stays in the turn's process group.
            child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(120)"])
            self.record("child", child_pid=child.pid)
        first = sys.stdin.readline()
        self.record("stdin", line=first.rstrip("\n"))
        source = self.scenario.get("api_key_source")
        if source is None:
            source = "ANTHROPIC_API_KEY" if os.environ.get("ANTHROPIC_API_KEY") else "none"
        self.emit(
            {
                "type": "system",
                "subtype": "init",
                "session_id": self.session_id,
                "model": self.model,
                "apiKeySource": source,
                "tools": [],
                "cwd": os.getcwd(),
            }
        )
        try:
            echo = json.loads(first)
            if isinstance(echo, dict):
                echo["isReplay"] = True
                echo["session_id"] = self.session_id
                self.emit(echo)
        except ValueError:
            pass
        result: dict[str, Any] | None = None
        for line in self.fixture():
            if line.get("type") == "result":
                result = line
                continue
            self.emit(line)
        self.gate("turn_gate")
        if self.scenario.get("hold"):
            self.sleep(600.0)
        if not self.scenario.get("no_result"):
            final = result or {
                "type": "result",
                "subtype": "success",
                "is_error": False,
                "result": "",
            }
            final.update(self.scenario.get("result_override", {}))
            self.emit(final)
        exit_code = int(self.scenario.get("exit_code", 0))
        if self.scenario.get("exit_without_eof"):
            self.record("turn_end", code=exit_code)
            return exit_code
        for line in sys.stdin:
            self.record("stdin", line=line.rstrip("\n"))
        self.record("turn_end", code=exit_code)
        return exit_code

    def sleep(self, seconds: float) -> None:
        self.stopping.wait(seconds)

    def gate(self, key: str) -> None:
        """Wait until the test creates the gate file of ``key`` (or a SIGINT stops the turn)."""
        path = self.scenario.get(key)
        if not path:
            return
        self.record("gate_wait", gate=key)
        # Only a bound for an orphaned process; a test releases the gate or ends the process.
        deadline = time.monotonic() + GATE_PATIENCE
        while not Path(path).exists() and time.monotonic() < deadline:
            if self.stopping.wait(0.01):
                return

    def on_sigint(self, *_: Any) -> None:
        self.emit(
            {
                "type": "result",
                "subtype": "error_during_execution",
                "is_error": True,
                "session_id": self.session_id,
                "result": "",
            }
        )
        self.record("turn_end", code=130, signal="SIGINT")
        os._exit(130)

    def fixture(self) -> list[dict[str, Any]]:
        name = self.scenario.get("fixture", "plain")
        out = []
        for raw in (FIXTURES / f"{name}.jsonl").read_text().splitlines():
            if not raw.strip():
                continue
            text = raw.replace("{{session_id}}", self.session_id).replace("{{model}}", self.model)
            line = json.loads(text)
            if line.get("type") == "fixture_meta":
                continue
            out.append(line)
        return out


@contextlib.contextmanager
def _no_sigint() -> Iterator[None]:
    """Hold SIGINT back during a write: its handler writes too, and a handler that ran inside
    a blocked flush of the same buffer would die on a reentrant call (on a slow machine)."""
    old = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGINT})
    try:
        yield
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, old)


def _after(argv: list[str], flag: str) -> str | None:
    if flag in argv:
        index = argv.index(flag)
        if index + 1 < len(argv):
            return argv[index + 1]
    return None


def main(scenario_path: str, capture_path: str) -> int:
    scenario = json.loads(Path(scenario_path).read_text())
    fake = Fake(scenario, Path(capture_path))
    args = sys.argv[1:]
    if args[:1] == ["--version"]:
        return fake.version()
    if args[:2] == ["auth", "status"]:
        return fake.auth_status()
    if args[:1] == ["--help"]:
        print("fake help")
        return 0
    if "-p" in args:
        return fake.turn()
    fake.record("unknown", argv=args)
    return 2


if __name__ == "__main__":  # pragma: no cover - the wrapper calls main
    sys.exit(main(sys.argv.pop(1), sys.argv.pop(1)))
