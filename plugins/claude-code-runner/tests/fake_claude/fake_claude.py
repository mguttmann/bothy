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
  With ``background_gate`` the turn starts a background job that runs until the test creates
  the gate file, and the process stays between turns meanwhile (`Fake.background_phase`).
  ``{{tmpdir}}`` and ``{{add_dir}}`` in a fixture, in ``result_override`` or in
  ``meta_result_text`` become the turn's ``TMPDIR`` and its ``--add-dir``.

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
PINNED = "2.1.288"
# quality.test.patience (bothy_sdk.testing._patience.PATIENCE); standard library only here.
GATE_PATIENCE = 180.0
BACKGROUND_TASK = "bgjob01"
"""The task ID of the scripted background job (``background_gate``)."""


class Fake:
    def __init__(self, scenario: dict[str, Any], capture: Path) -> None:
        self.scenario = scenario
        self.capture = capture
        self.lock = threading.RLock()
        self.session_id = ""
        self.model = ""
        self.stopping = threading.Event()
        self.job_running = False

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

    def turn(self) -> int:  # noqa: PLR0911, PLR0912 - one branch per scripted behaviour
        argv = sys.argv[1:]
        prompt_file = _after(argv, "--append-system-prompt-file")
        system_prompt = (
            Path(prompt_file).read_text() if prompt_file and Path(prompt_file).is_file() else None
        )
        resume = _after(argv, "--resume")
        self.session_id = _after(argv, "--session-id") or resume or ""
        self.model = _after(argv, "--model") or ""
        index = sum(1 for c in _capture(self.capture) if c["kind"] == "turn_start")
        self.scenario = {
            **self.scenario,
            **self.scenario.get("per_process", {}).get(str(index), {}),
        }
        self.record(
            "turn_start",
            program=sys.argv[0],
            argv=argv,
            cwd=os.getcwd(),
            env=dict(os.environ),
            system_prompt=system_prompt,
            index=index,
        )
        signal.signal(
            signal.SIGINT, signal.SIG_IGN if self.scenario.get("ignore_sigint") else self.on_sigint
        )
        if self.scenario.get("ignore_sigterm"):
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
        if self.scenario.get("spawn_child"):
            # A background job of a tool call: it stays in the turn's process group.
            code = "import time; time.sleep(120)"
            if self.scenario.get("child_ignores_sigint"):  # like an `&` job of a shell
                code = "import signal, time; signal.signal(signal.SIGINT, signal.SIG_IGN); " + code
            child = subprocess.Popen([sys.executable, "-c", code])
            self.record("child", child_pid=child.pid)
        if self.scenario.get("stderr_canary"):
            with _no_sigint():
                sys.stderr.write(str(self.scenario["stderr_canary"]) + "\n")
                sys.stderr.flush()
        if self.scenario.get("fail_before_init"):
            self.record("turn_end", code=1, refused="fail_before_init")
            return 1
        refused = self.session_check(resume is not None)
        if refused is not None:
            return refused
        first = sys.stdin.readline()
        self.record("stdin", line=first.rstrip("\n"))
        if _text_of(first) == "/compact":
            return self.compact()
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
        self.remember_session()
        self.echo(first)
        lines = _StdinLines(self)
        # Like the pinned CLI: a line is echoed when a turn takes it (at the start of its own
        # turn, or when it is folded into a running turn), not when it is read; a line still
        # queued has no echo ("on_consume"). "on_read" echoes at once.
        echo_mode = self.scenario.get("echo", "on_consume")
        if echo_mode == "on_read":
            lines.on_line = self.echo
        lines.start()
        error = self.scenario.get("api_error")
        result: dict[str, Any] | None = None
        played = 0
        for line in self.fixture():
            if line.get("type") == "result":
                result = line
                continue
            if error and played >= int(self.scenario.get("api_error_after", 0)):
                break
            self.emit(line)
            played += 1
            if played == self.scenario.get("gate_after_lines"):
                self.gate(
                    "turn_gate"
                )  # inside the turn, for example between a tool call and its result
        if "gate_after_lines" not in self.scenario:
            self.gate("turn_gate")
        if error:
            return self.api_error(_meta(str(error))["error_text"], lines)
        if self.scenario.get("hold"):
            self.sleep(600.0)
        # Like the CLI: user lines queued while the turn ran are folded into it, and its
        # result names every input it consumed in user_message_uuids.
        folded = min(len(lines.lines), int(self.scenario.get("fold_queued", 0)))
        if folded:
            self.record("folded", count=folded)
            if echo_mode == "on_consume":
                for line in lines.lines[:folded]:
                    self.echo(line)
        consumed = [_uuid_of(x) for x in [first, *lines.lines[:folded]]]
        if self.scenario.get("background_gate"):
            self.start_background()
        if not self.scenario.get("no_result"):
            final = dict(
                result or {"type": "result", "subtype": "success", "is_error": False, "result": ""}
            )
            final["user_message_uuids"] = [u for u in consumed if u]
            final.update(self.scenario.get("result_override", {}))
            if isinstance(final.get("result"), str):
                final["result"] = _expand(final["result"])
            self.emit(final)
        if self.scenario.get("meta_result"):
            # Like the CLI: a synthetic turn (a background task's notification) ends with a
            # success result that names no input (no user_message_uuid(s)).
            self.record("meta_result")
            self.emit(
                {
                    "type": "result",
                    "subtype": "success",
                    "is_error": False,
                    "session_id": self.session_id,
                    "result": _expand(
                        str(self.scenario.get("meta_result_text", "Background job finished."))
                    ),
                }
            )
        exit_code = int(self.scenario.get("exit_code", 0))
        if self.scenario.get("background_gate"):
            return self.background_phase(lines, folded, exit_code)
        if self.scenario.get("exit_without_eof"):
            self.record("turn_end", code=exit_code)
            return exit_code
        # One result per further input line, in order, while stdin is open (09 18.5).
        answered = folded
        while (line := lines.next(answered)) is not None:
            answered += 1
            if echo_mode in ("after_result", "on_consume"):
                self.echo(line)
            if echo_mode == "never":
                continue
            if answered - folded <= int(self.scenario.get("meta_folds", 0)):
                # Like 2.1.259: the line was folded into a notification (meta) turn between
                # tool rounds; its echo came, and the turn's result names no input.
                self.record("meta_fold", line=_uuid_of(line))
                meta = self.followup_result(answered, result)
                meta["result"] = "Background job finished; also did the follow-up."
                self.emit(meta)
                continue
            if self.scenario.get("api_error_on_followup"):
                self.emit_api_error(
                    _meta(str(self.scenario["api_error_on_followup"]))["error_text"]
                )
                continue
            self.emit(
                {**self.followup_result(answered, result), "user_message_uuids": [_uuid_of(line)]}
            )
        self.gate("end_gate")  # stdin is closed: the process still runs until the gate opens
        self.record("turn_end", code=exit_code)
        return exit_code

    # ---- FIX-runner-responsive-turns: a background job outlives the turn ---------------------

    def init_line(self) -> dict[str, Any]:
        return {
            "type": "system",
            "subtype": "init",
            "session_id": self.session_id,
            "model": self.model,
            "apiKeySource": "ANTHROPIC_API_KEY" if os.environ.get("ANTHROPIC_API_KEY") else "none",
            "tools": [],
            "cwd": os.getcwd(),
        }

    def start_background(self) -> None:
        """The turn starts a background job (Bash with ``run_in_background``) before its result."""
        self.record("background_started")
        self.emit(
            {
                "type": "system",
                "subtype": "task_started",
                "session_id": self.session_id,
                "task_id": BACKGROUND_TASK,
                "tool_use_id": "toolu_bg",
                "task_type": "local_bash",
                "description": "Run the test suite",
            }
        )
        self.emit(
            {
                "type": "system",
                "subtype": "background_tasks_changed",
                "session_id": self.session_id,
                "tasks": [
                    {
                        "task_id": BACKGROUND_TASK,
                        "task_type": "local_bash",
                        "description": "Run the test suite",
                    }
                ],
            }
        )

    def background_phase(self, lines: _StdinLines, answered: int, exit_code: int) -> int:
        """Between turns while the background job runs, like the pinned CLI (09 18.5).

        * A further stdin line starts a turn of its own at once (``system/init``, echo,
          ``result`` naming the line); its answer says whether the job still runs.
        * The job ends when the test creates ``background_gate``: the task list empties, the
          notification comes and the CLI starts a turn by itself whose ``result`` names no
          input (``background_text``).
        * The end of stdin while the job runs stops the job (``background_killed``): the CLI
          ends its background work when its input ends.
        * With ``background_error_gate``: once the test creates it, the CLI starts a turn by
          itself (a task's notification) that fails with the API error of the fixture named by
          ``background_api_error``, as on 2026-11-01 when the host's login failed briefly.
        * With ``background_survives_eof``: the end of stdin does not stop the job; the process
          stays until the job ends or a signal comes, as the pinned CLI did on 2026-11-01 for
          about ten minutes after a failed turn (``eof_with_background``).
        """
        gate = Path(str(self.scenario["background_gate"]))
        error_gate = self.scenario.get("background_error_gate")
        running = True
        self.job_running = True
        eof_logged = False
        deadline = time.monotonic() + 2 * GATE_PATIENCE  # only a bound for an orphaned process
        while time.monotonic() < deadline:
            if running and gate.exists():
                running = False
                self.job_running = False
                self.background_done()
            if error_gate and Path(str(error_gate)).exists():
                error_gate = None
                self.record("background_turn_failed")
                self.emit(self.init_line())
                self.emit_api_error(_meta(str(self.scenario["background_api_error"]))["error_text"])
            line, done = lines.wait(answered, 0.01)
            if line is not None:
                answered += 1
                self.emit(self.init_line())
                self.echo(line)
                state = "still running" if running else "finished"
                self.emit(
                    {
                        **self.followup_result(answered, None),
                        "result": f"Status: the background job is {state}.",
                        "user_message_uuids": [_uuid_of(line)],
                    }
                )
                continue
            if done and running and self.scenario.get("background_survives_eof"):
                if not eof_logged:
                    eof_logged = True
                    self.record("eof_with_background")
                self.stopping.wait(0.01)  # poll for the job's gate or a signal
                continue
            if done:
                break
        if running:
            self.record("background_killed")
            self.emit(
                {
                    "type": "system",
                    "subtype": "task_notification",
                    "session_id": self.session_id,
                    "task_id": BACKGROUND_TASK,
                    "status": "stopped",
                    "summary": "Run the test suite",
                }
            )
        self.gate("end_gate")
        self.record("turn_end", code=exit_code)
        return exit_code

    def background_done(self) -> None:
        self.record("background_done")
        self.emit(
            {
                "type": "system",
                "subtype": "background_tasks_changed",
                "session_id": self.session_id,
                "tasks": [],
            }
        )
        self.emit(
            {
                "type": "system",
                "subtype": "task_notification",
                "session_id": self.session_id,
                "task_id": BACKGROUND_TASK,
                "status": "completed",
                "summary": 'Background command "Run the test suite" completed (exit code 0)',
            }
        )
        self.emit(self.init_line())
        self.emit(
            {
                "type": "result",
                "subtype": "success",
                "is_error": False,
                "session_id": self.session_id,
                "result": str(self.scenario.get("background_text", "The suite passed.")),
            }
        )

    # ---- M2-35: sessions, compaction, errors and follow-ups ----------------------------------

    def store(self) -> Path:
        base = os.environ.get("CLAUDE_CONFIG_DIR") or str(Path(os.environ["HOME"]) / ".claude")
        return Path(base) / "fake-sessions"

    def session_check(self, resume: bool) -> int | None:
        """With ``sessions``: an unknown ``--resume`` or a known ``--session-id`` is refused."""
        if not self.scenario.get("sessions"):
            return None
        known = (self.store() / self.session_id).exists() or self.session_id in self.scenario.get(
            "known_sessions", []
        )
        if resume == known:
            return None
        name = "session_missing" if resume else "session_exists"
        text = _meta(name)["error_text"].replace("{{session_id}}", self.session_id)
        with _no_sigint():
            sys.stderr.write(text + "\n")
            sys.stderr.flush()
        if resume:  # with stream-json the CLI also writes the text as an error result
            self.emit(
                {
                    "type": "result",
                    "subtype": "error_during_execution",
                    "is_error": True,
                    "session_id": self.session_id,
                    "errors": [text],
                }
            )
        self.record("turn_end", code=1, refused=name)
        return 1

    def remember_session(self) -> None:
        if self.scenario.get("sessions"):
            self.store().mkdir(parents=True, exist_ok=True)
            (self.store() / self.session_id).write_text("")

    def compact(self) -> int:
        """``/compact`` as the only input line: a compaction ``result`` (or a scripted failure)."""
        self.record("compact")
        self.emit(
            {
                "type": "system",
                "subtype": "init",
                "session_id": self.session_id,
                "model": self.model,
                "apiKeySource": "ANTHROPIC_API_KEY"
                if os.environ.get("ANTHROPIC_API_KEY")
                else "none",
                "tools": [],
                "cwd": os.getcwd(),
            }
        )
        self.gate("compact_gate")
        failed = bool(self.scenario.get("compact_fails"))
        if not failed:
            self.emit(
                {
                    "type": "system",
                    "subtype": "compact_boundary",
                    "session_id": self.session_id,
                    "compact_metadata": {"trigger": "manual", "pre_tokens": 190000},
                }
            )
        # Like the pinned CLI: a failed compaction also ends with a success result (no
        # compact_boundary, no turn), whose text says that it failed.
        self.emit(
            {
                "type": "result",
                "subtype": "success",
                "is_error": False,
                "num_turns": 0 if failed else 1,
                "session_id": self.session_id,
                "result": "Compaction failed" if failed else "COMPACTION-OUTPUT-NEVER-POSTED",
                "modelUsage": _usage(self.model, 400, 0.004),
            }
        )
        for line in sys.stdin:
            self.record("stdin", line=line.rstrip("\n"))
        self.record("turn_end", code=0)
        return 0

    def api_error(self, text: str, lines: _StdinLines) -> int:
        """An API error as the CLI writes it: an assistant message, then an error result."""
        self.emit_api_error(text)
        lines.read_all()
        self.record("turn_end", code=1)
        return 1

    def emit_api_error(self, text: str) -> None:
        self.emit(
            {
                "type": "assistant",
                "session_id": self.session_id,
                "parent_tool_use_id": None,
                "is_api_error_message": True,
                "message": {
                    "id": "msg_err",
                    "model": "<synthetic>",
                    "role": "assistant",
                    "content": [{"type": "text", "text": text}],
                },
            }
        )
        self.emit(
            {
                "type": "result",
                "subtype": "success",
                "is_error": True,
                "session_id": self.session_id,
                "result": text,
                "modelUsage": _usage(self.model, 300, 0.003),
            }
        )

    def echo(self, raw: str) -> None:
        try:
            doc = json.loads(raw)
        except ValueError:
            return
        if isinstance(doc, dict):
            doc["isReplay"] = True
            doc["session_id"] = self.session_id
            self.emit(doc)

    def followup_result(self, answered: int, first: dict[str, Any] | None) -> dict[str, Any]:
        base = (first or {}).get("modelUsage", {}).get(self.model, {})
        tokens = int(base.get("inputTokens", 100)) + 1000 * answered
        cost = float(base.get("costUSD", 0.001)) + 0.01 * answered
        return {
            "type": "result",
            "subtype": "success",
            "is_error": False,
            "session_id": self.session_id,
            "result": f"Follow-up {answered} handled.",
            "modelUsage": _usage(self.model, tokens, cost),
        }

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
        if self.job_running:
            self.record("background_killed", signal="SIGINT")
        self.emit(
            {
                "type": "result",
                "subtype": "error_during_execution",
                "is_error": True,
                "session_id": self.session_id,
                "result": "",
                "modelUsage": _usage(self.model, 50, 0.0005),
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
            text = _expand(text)
            line = json.loads(text)
            if line.get("type") == "fixture_meta":
                continue
            out.append(line)
        return out


class _StdinLines:
    """Reads the further stdin lines in a thread, so a turn can echo them while it works."""

    def __init__(self, fake: Fake) -> None:
        self.fake = fake
        self.lines: list[str] = []
        self.done = False
        self.changed = threading.Condition()
        self.on_line: Any = None
        self.thread = threading.Thread(target=self._run, daemon=True)

    def start(self) -> None:
        self.thread.start()

    def _run(self) -> None:
        for line in sys.stdin:
            self.fake.record("stdin", line=line.rstrip("\n"))
            if self.on_line is not None:
                self.on_line(line)
            with self.changed:
                self.lines.append(line)
                self.changed.notify_all()
        with self.changed:
            self.done = True
            self.changed.notify_all()

    def next(self, index: int) -> str | None:
        """The further line number ``index`` once read, or ``None`` at the end of stdin."""
        with self.changed:
            self.changed.wait_for(lambda: self.done or len(self.lines) > index, GATE_PATIENCE)
            return self.lines[index] if len(self.lines) > index else None

    def wait(self, index: int, timeout: float) -> tuple[str | None, bool]:
        """The further line number ``index`` if it comes within ``timeout``, and the end flag."""
        with self.changed:
            self.changed.wait_for(lambda: self.done or len(self.lines) > index, timeout)
            if len(self.lines) > index:
                return self.lines[index], False
            return None, self.done

    def read_all(self) -> list[str]:
        """Wait for the end of stdin (the runner closes it) and return every further line."""
        with self.changed:
            self.changed.wait_for(lambda: self.done, GATE_PATIENCE)
            return list(self.lines)


def _expand(text: str) -> str:
    """``{{tmpdir}}`` and ``{{add_dir}}``: the turn's ``TMPDIR`` and its ``--add-dir`` (the
    thread's file folder), so an answer can attach a scratch or an incoming file (M3-12)."""
    return text.replace("{{tmpdir}}", os.environ.get("TMPDIR", "")).replace(
        "{{add_dir}}", _after(sys.argv[1:], "--add-dir") or ""
    )


def _capture(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        return []
    return [json.loads(x) for x in path.read_text().splitlines() if x.strip()]


def _uuid_of(raw: str) -> str:
    try:
        doc = json.loads(raw)
    except ValueError:
        return ""
    return str(doc.get("uuid") or "") if isinstance(doc, dict) else ""


def _text_of(raw: str) -> str:
    try:
        doc = json.loads(raw)
        return str(doc["message"]["content"][0]["text"])
    except (ValueError, KeyError, IndexError, TypeError):
        return ""


def _meta(name: str) -> dict[str, Any]:
    meta: dict[str, Any] = json.loads((FIXTURES / f"{name}.jsonl").read_text().splitlines()[0])
    return meta


def _usage(model: str, tokens: int, cost: float) -> dict[str, Any]:
    return {
        model: {
            "inputTokens": tokens,
            "outputTokens": 10,
            "cacheReadInputTokens": 0,
            "cacheCreationInputTokens": 0,
            "costUSD": cost,
            "contextWindow": 200000,
        }
    }


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
