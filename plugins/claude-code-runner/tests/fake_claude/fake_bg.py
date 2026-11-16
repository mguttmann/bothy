# SPDX-License-Identifier: Apache-2.0
"""The background side of Fake-Claude: ``claude --bg`` and its control commands (09 16.1).

Standard library only; `fake_claude.main` calls it for the commands of background sessions:

* ``[--resume <id>] --bg <flags>`` with the prompt on stdin: prints the launch line
  ``backgrounded · <short id> · <name>``, starts the fake supervisor when none runs
  (``Starting background service…`` on stderr) and a detached player, the session process.
  The refusals of the pinned CLI are scripted (``bg_refuse``: ``untrusted``, ``bypass``,
  ``error``), a launch without a process (``bg_no_process``), and the copy the CLI starts for
  a resume whose session still has an entry.
* ``agents --json --all``, ``stop <id>``, ``rm <id>`` and ``daemon status`` answer from the
  state file ``<config dir>/fake-bg/state.json`` of the fake ``HOME``.

The player appends the session transcript ``<config dir>/projects/<folder>/<session id>.jsonl``
with the records of the pinned CLI the runner reads (``tests/fixtures/recording/background_*``
holds their members; a test compares) and with records it must skip, among them the three
that hold account identifiers, here with `ACCOUNT_CANARY`. It plays the stream fixture of the
scenario as transcript records, listens on the inbox socket of ``--messaging-socket-path``,
takes a line after the running tool call (``remove`` with ``absorbed_mid_turn`` and the
attachment ``queued_command``) or as a turn of its own, and writes the session totals
(``cost-state``) when ``stop`` ends it. Like the print side it waits on gates the test opens,
never on a fixed time.

The socket takes a line of any program, as the CLI's does, and the player decides like the
pinned CLI what becomes of it (`Player.inbound`): by ``crossSessionInbound`` of the settings
when the launch set one, else per message by the permission classes: a session that bypasses
permission prompts takes only a line whose envelope states ``bypass``, every other session
every line that does not state it. A line that is held is noted in the transcript and never
delivered; when the rule per message held it (not a value of the setting), the session list
then shows the session as waiting for a permission prompt.

The session's environment is the supervisor's (the environment of the launch that started
it) with the ``env`` block of ``--settings`` on top, as in the pinned CLI, where the launching
command's own environment does not reach the session.
"""

from __future__ import annotations

import contextlib
import fcntl
import json
import os
import re
import signal
import socket
import subprocess
import sys
import threading
import time
import uuid
from collections.abc import Iterator
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

ACCOUNT_CANARY = "ACCOUNT-CANARY-7c1d"
"""A value in the records that hold account identifiers; no file of the runner may carry it."""

LOGIN_BETA = "oauth-2025-04-20"
"""The beta of a request that uses the stored login, as the pinned CLI names it."""

ENVELOPE = re.compile(
    r'^<cross-session-message from-mode="(bypass|prompting)">\n[\s\S]*\n</cross-session-message>$'
)
"""The envelope in which a sender states its permission class, as the pinned CLI reads it."""

PEER_LEAD = "Another Claude session sent a message:\n"
PEER_TAIL = (
    "\n\nThis came from another Claude session - not typed by your user, but very likely "
    "working on their behalf."
)
SUPERVISOR_LINGER = 0.3
"""How long the fake supervisor outlives its last session process (seconds)."""


def base_dir() -> Path:
    env = os.environ
    return Path(env.get("CLAUDE_CONFIG_DIR") or str(Path(env["HOME"]) / ".claude"))


def _state_dir() -> Path:
    path = base_dir() / "fake-bg"
    path.mkdir(parents=True, exist_ok=True)
    return path


@contextlib.contextmanager
def state() -> Iterator[dict[str, Any]]:
    """The state file under an exclusive lock; what the block leaves is written back."""
    folder = _state_dir()
    with (folder / "state.lock").open("a+") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        path = folder / "state.json"
        try:
            doc = json.loads(path.read_text())
        except (OSError, ValueError):
            doc = {}
        doc.setdefault("sessions", {})
        before = json.dumps(doc, sort_keys=True)
        yield doc
        if json.dumps(doc, sort_keys=True) != before:
            tmp = path.with_suffix(".tmp")
            tmp.write_text(json.dumps(doc))
            tmp.replace(path)


def alive(pid: object) -> bool:
    if not isinstance(pid, int) or pid <= 1:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _after(argv: list[str], flag: str) -> str | None:
    if flag in argv:
        index = argv.index(flag)
        if index + 1 < len(argv):
            return argv[index + 1]
    return None


def _now() -> str:
    return datetime.now(UTC).isoformat().replace("+00:00", "Z")


def transcript_path(cwd: str, session_id: str) -> Path:
    folder = re.sub(r"[^A-Za-z0-9]", "-", cwd)
    return base_dir() / "projects" / folder / f"{session_id}.jsonl"


def _read_json(path: str | None) -> Any:
    if not path:
        return None
    try:
        return json.loads(path if path.lstrip().startswith("{") else Path(path).read_text())
    except (OSError, ValueError):
        return None


# ---- the launch command ------------------------------------------------------------------------


def launch(fake: Any) -> int:  # noqa: PLR0912 - one branch per scripted behaviour
    argv = sys.argv[1:]
    prompt = sys.stdin.read()
    index = sum(1 for c in _capture(fake.capture) if c["kind"] == "bg_launch")
    scenario = {**fake.scenario, **fake.scenario.get("per_launch", {}).get(str(index), {})}
    settings = _read_json(_after(argv, "--settings"))
    prompt_file = _after(argv, "--append-system-prompt-file")
    fake.record(
        "bg_launch",
        argv=argv,
        cwd=os.getcwd(),
        env=dict(os.environ),
        prompt=prompt,
        settings=settings,
        system_prompt=Path(prompt_file).read_text()
        if prompt_file and Path(prompt_file).is_file()
        else None,
        index=index,
    )
    refuse = scenario.get("bg_refuse")
    if refuse == "untrusted":
        sys.stderr.write(
            f"Workspace not trusted. Run `claude` in {os.getcwd()} once and accept the trust "
            "prompt, then retry.\n"
        )
        return 1
    if refuse == "bypass":
        sys.stderr.write(
            "--bg with bypassPermissions requires accepting the disclaimer first. Run "
            "`claude --dangerously-skip-permissions` once interactively.\n"
        )
        return 1
    if refuse == "error":
        sys.stderr.write("error: background sessions are not available\n")
        return 1
    resume = _after(argv, "--resume")
    name = _after(argv, "--name") or ""
    session_id = resume or str(uuid.uuid4())
    notes: list[str] = []
    started_service = False
    with state() as doc:
        supervisor = doc.get("supervisor")
        if not (supervisor and alive(supervisor.get("pid"))) and "bg_supervisor" not in scenario:
            # The first launch starts the supervisor, with the launch environment.
            process = subprocess.Popen(
                [sys.argv[0], "--bg-supervisor"],
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                start_new_session=True,
            )
            doc["supervisor"] = {
                "pid": process.pid,
                "version": scenario.get("version", fake.pinned),
                "env": dict(os.environ),
            }
            started_service = True
        if resume is not None:
            existing = doc["sessions"].get(resume[:8])
            if existing is not None or scenario.get("bg_copy"):
                # Like the pinned CLI: a session that still has an entry is not continued
                # in place by a command with flags; the command starts a copy.
                session_id = str(uuid.uuid4())
                running = existing is not None and alive(existing.get("pid"))
                if running or existing is None:
                    notes.append(
                        f"note: session {resume[:8]} is already running in the background, so "
                        f"this started a copy as {session_id[:8]}. `claude attach {resume[:8]}` "
                        "opens the original."
                    )
                else:
                    notes.append(
                        f"note: background session {resume[:8]} keeps its own saved options, "
                        f"so the flags you passed started a copy as {session_id[:8]}. Without "
                        f"flags, the same command continues {resume[:8]} itself."
                    )
        short = session_id[:8]
        entry: dict[str, Any] = {
            "id": short,
            "cwd": os.getcwd(),
            "kind": "background",
            "startedAt": int(time.time() * 1000),
            "sessionId": session_id,
            "name": name,
            "state": "failed" if scenario.get("bg_no_process") else "working",
        }
        doc["sessions"][short] = entry
        session_env = dict((doc.get("supervisor") or {}).get("env") or os.environ)
    if isinstance(settings, dict) and isinstance(settings.get("env"), dict):
        session_env.update({str(k): str(v) for k, v in settings["env"].items()})
    if not scenario.get("bg_silent"):  # else: exit 0 with an output the runner cannot read
        print(f"backgrounded · {short} · {name}")
        for command, what in (
            ("agents            ", "list sessions"),
            (f"attach {short}   ", "open in this terminal"),
            (f"logs {short}     ", "show recent output"),
            (f"stop {short}     ", "stop this session"),
        ):
            print(f"  claude {command} {what}")
        if started_service:
            sys.stderr.write("Starting background service…\n")
        for note in notes:
            sys.stderr.write(note + "\n")
    sys.stdout.flush()
    sys.stderr.flush()
    if scenario.get("bg_no_process"):
        return 0
    spec = {
        "argv": argv,
        "prompt": prompt,
        "settings": settings,
        "session_id": session_id,
        "resumed": resume is not None,
        "index": index,
    }
    (_state_dir() / f"{short}.spec.json").write_text(json.dumps(spec))
    subprocess.Popen(
        [sys.argv[0], "--bg-player", short],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=(_state_dir() / f"{short}.stderr").open("ab"),
        env=session_env,
        start_new_session=True,
    )
    return 0


def supervisor(fake: Any) -> int:
    """The fake supervisor: lives while a session process lives, and a moment longer."""
    fake.record("bg_supervisor", pgid=os.getpgid(0))
    quiet_since: float | None = None
    deadline = time.monotonic() + 4 * fake.patience
    while time.monotonic() < deadline:
        time.sleep(0.02)
        with state() as doc:
            busy = any(alive(e.get("pid")) for e in doc["sessions"].values())
            if busy:
                quiet_since = None
                continue
            quiet_since = quiet_since if quiet_since is not None else time.monotonic()
            if time.monotonic() - quiet_since >= SUPERVISOR_LINGER:
                if (doc.get("supervisor") or {}).get("pid") == os.getpid():
                    doc.pop("supervisor")
                return 0
    return 0


# ---- the control commands ----------------------------------------------------------------------


def agents(fake: Any) -> int:
    fake.record("bg_agents", argv=sys.argv[1:])
    entries: list[dict[str, Any]] = []
    with state() as doc:
        for entry in doc["sessions"].values():
            shown = {k: v for k, v in entry.items() if k != "totals"}
            if "pid" in shown and not alive(shown["pid"]):
                # A process that ended without a stop: the entry loses its process.
                for key in ("pid", "status", "waitingFor"):
                    shown.pop(key, None)
                shown["state"] = "failed"
            entries.append(shown)
    entries += list(fake.scenario.get("bg_foreign_entries", []))
    if fake.scenario.get("bg_agents_raw") is not None:
        sys.stdout.write(str(fake.scenario["bg_agents_raw"]))
        return int(fake.scenario.get("bg_agents_exit", 0))
    print(json.dumps(entries, indent=2))
    return 0


def stop(fake: Any) -> int:
    short = (sys.argv[2:] or [""])[0]
    fake.record("bg_stop", id=short)
    with state() as doc:
        entry = doc["sessions"].get(short)
        pid = entry.get("pid") if entry else None
    if entry is None:
        sys.stderr.write(
            f"No job matching '{short}'. Run 'claude agents' to list running sessions.\n"
        )
        return 1
    if alive(pid):
        with contextlib.suppress(ProcessLookupError):
            os.kill(pid, signal.SIGUSR1)
        deadline = time.monotonic() + (0.2 if fake.scenario.get("bg_ignore_stop") else 60.0)
        while alive(pid) and time.monotonic() < deadline:
            time.sleep(0.01)
    with state() as doc:
        entry = doc["sessions"].get(short)
        if entry is not None and not alive(entry.get("pid")):
            for key in ("pid", "status", "waitingFor"):
                entry.pop(key, None)
            entry["state"] = "stopped"
    print(f"stopped {short}")
    return 0


def remove(fake: Any) -> int:
    short = (sys.argv[2:] or [""])[0]
    fake.record("bg_rm", id=short)
    with state() as doc:
        entry = doc["sessions"].get(short)
        if entry is not None and alive(entry.get("pid")):
            sys.stderr.write(f"kept {short} - the session is still running\n")
            return 1
        if entry is not None:
            del doc["sessions"][short]
    if entry is None:
        sys.stderr.write(f"No job matching '{short}'\n")
        return 1
    print(f"removed {short}")
    return 0


def daemon_status(fake: Any) -> int:
    fake.record("bg_daemon", argv=sys.argv[1:])
    forced = fake.scenario.get("bg_supervisor")
    with state() as doc:
        found = forced or doc.get("supervisor")
    if fake.scenario.get("bg_daemon_raw") is not None:
        sys.stdout.write(str(fake.scenario["bg_daemon_raw"]))
        return int(fake.scenario.get("bg_daemon_exit", 0))
    if not found or not alive(found.get("pid")):
        print("not running\n\nbg sessions:\n  control.sock: unreachable")
        return 1
    print(f"pid:     {found['pid']}")
    print(f"version: {found.get('version', fake.pinned)}")
    print("uptime:  1s")
    print("origin:  transient - started on-demand by `claude --bg`")
    return 0


# ---- the session process -----------------------------------------------------------------------


class Player:
    """One session process: it plays the scenario as transcript records and takes inbox lines."""

    def __init__(self, fake: Any, short: str) -> None:
        self.fake = fake
        self.short = short
        self.spec = json.loads((_state_dir() / f"{short}.spec.json").read_text())
        self.argv: list[str] = self.spec["argv"]
        self.session_id: str = self.spec["session_id"]
        self.model = _after(self.argv, "--model") or ""
        fake.session_id, fake.model = self.session_id, self.model
        fake.scenario = {
            **fake.scenario,
            **fake.scenario.get("per_launch", {}).get(str(self.spec["index"]), {}),
        }
        self.scenario: dict[str, Any] = fake.scenario
        self.path = transcript_path(os.getcwd(), self.session_id)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.lock = threading.RLock()
        self.inbox: list[dict[str, Any]] = []
        self.arrived = threading.Condition(self.lock)
        self.stopped = threading.Event()
        self.last_uuid: str | None = None
        self.totals = self._restored_totals()
        self.requests = 0
        self.followups = 0
        self.socket_path = _after(self.argv, "--messaging-socket-path")
        self.server: socket.socket | None = None
        self.inbound_setting = (self.spec.get("settings") or {}).get("crossSessionInbound")
        self.bypasses = _after(self.argv, "--permission-mode") == "bypassPermissions"
        self.held = 0
        self.job_running = False
        self.turn_open = False
        self._status: tuple[str, str | None] | None = None
        sys.argv[1:] = self.argv  # the fixtures expand {{add_dir}} from the launch's arguments

    # ---- records -------------------------------------------------------------------------------

    def write(self, record: dict[str, Any]) -> None:
        with self.lock, self.path.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(record) + "\n")

    def _envelope(self) -> dict[str, Any]:
        return {
            "timestamp": _now(),
            "sessionKind": "bg",
            "userType": "external",
            "entrypoint": "cli",
            "cwd": os.getcwd(),
            "sessionId": self.session_id,
            "version": self.fake.pinned,
        }

    def record(self, kind: str, **members: Any) -> str:
        ident = str(members.pop("uuid", None) or uuid.uuid4())
        self.write(
            {
                "parentUuid": self.last_uuid,
                "isSidechain": False,
                "type": kind,
                "uuid": ident,
                **members,
                **self._envelope(),
            }
        )
        self.last_uuid = ident
        return ident

    def plain(self, kind: str, **members: Any) -> None:
        record = {"type": kind, "sessionId": self.session_id, "timestamp": _now(), **members}
        if kind == "cost-state":
            del record["timestamp"]  # the pinned CLI writes the totals without one
        self.write(record)

    def preamble(self) -> None:
        """Records the runner skips, among them those that hold account identifiers."""
        self.plain("custom-title", customTitle=_after(self.argv, "--name") or "")
        self.plain("mode", mode="normal")
        self.plain("session-owner", accountUuid=ACCOUNT_CANARY, organizationUuid=ACCOUNT_CANARY)
        for kind in ("session_context", "credential_org"):
            self.record("attachment", attachment={"type": kind, "account": ACCOUNT_CANARY})

    def user(self, text: str, origin: dict[str, Any], ident: str | None = None) -> None:
        self.turn_open = True
        self.record(
            "user",
            uuid=ident,
            message={"role": "user", "content": text},
            origin=origin,
            promptSource="typed" if origin.get("kind") == "human" else "system",
        )

    def request(self, *, credential: bool = True) -> None:
        """One request record, as the scenario scripts its login beta (the check of 18.3).

        ``bg_no_login``: no request lists the beta; ``bg_login_beta``: another name stands in
        its place (a renamed beta); ``bg_login_requests``: only the first so many requests
        list it; ``bg_no_request``: the session writes no request record at all;
        ``bg_request_records``: it writes one only for the first so many requests. Without a
        ``credential`` the record lists no login beta, as the record of the pinned CLI for a
        request it never sends because its login is gone.
        """
        self.requests += 1
        records = self.scenario.get("bg_request_records")
        if self.scenario.get("bg_no_request") or (records is not None and self.requests > records):
            return
        betas = ["claude-code-20250219", "interleaved-thinking-2025-05-14"]
        limit = self.scenario.get("bg_login_requests")
        listed = not self.scenario.get("bg_no_login") and (limit is None or self.requests <= limit)
        if credential and listed:
            betas.append(str(self.scenario.get("bg_login_beta", LOGIN_BETA)))
        self.plain(
            "api-request",
            id=str(uuid.uuid4()),
            version=self.fake.pinned,
            querySource="repl_main_thread",
            params={"model": self.model, "betas": betas, "max_tokens": 128000},
        )

    def assistant(self, message: dict[str, Any], **members: Any) -> None:
        """One record per content block, sharing the message id and usage, like the CLI."""
        blocks = message.get("content") or [{"type": "text", "text": ""}]
        for block in blocks:
            self.record("assistant", message={**message, "content": [block]}, **members)

    def text(self, value: str, ident: str | None = None) -> None:
        self.request()
        self.assistant(
            {
                "id": ident or f"msg_{uuid.uuid4().hex[:12]}",
                "type": "message",
                "role": "assistant",
                "model": self.model,
                "content": [{"type": "text", "text": value}],
                "stop_reason": "end_turn",
                "usage": {"input_tokens": 100, "output_tokens": 5},
            }
        )

    def api_error(self, text: str, members: dict[str, Any] | None) -> None:
        named = dict(members or {})
        # Like the pinned CLI: a session whose login is gone writes the record of a request
        # without the login beta, sends nothing, and writes the error itself.
        self.request(credential=named.get("error") != "authentication_failed")
        extra: dict[str, Any] = {"isApiErrorMessage": True, "error": named.get("error")}
        if "api_error_params" in named:
            extra["apiErrorParams"] = named["api_error_params"]
        if "api_error_status" in named:
            extra["apiErrorStatus"] = named["api_error_status"]
        self.assistant(
            {
                "id": str(uuid.uuid4()),
                "type": "message",
                "role": "assistant",
                "model": "<synthetic>",
                "content": [{"type": "text", "text": text}],
                "stop_reason": "stop_sequence",
                "usage": {"input_tokens": 0, "output_tokens": 0},
            },
            **extra,
        )

    def turn_end(self) -> None:
        # Like the pinned CLI: the session list shows the state after the turn (``busy`` only
        # while background work runs) by the time the turn's last record is written.
        self.status("busy" if self.job_running else "idle")
        self.record("system", subtype="turn_duration", durationMs=1, messageCount=1, isMeta=False)
        self.turn_open = False

    # ---- the session list ------------------------------------------------------------------------

    def status(self, value: str, waiting_for: str | None = None) -> None:
        if self.held and waiting_for is None:
            # Like the pinned CLI: a held message waits for an approval, whatever else the
            # session does meanwhile.
            value, waiting_for = "waiting", "permission prompt"
        if self._status == (value, waiting_for):
            return
        self._status = (value, waiting_for)
        with state() as doc:
            entry = doc["sessions"].get(self.short)
            if entry is None:
                return
            entry["pid"] = os.getpid()
            entry["status"] = value
            entry.pop("waitingFor", None)
            if waiting_for is not None:
                entry["waitingFor"] = waiting_for

    def _restored_totals(self) -> dict[str, Any]:
        totals: dict[str, Any] = {}
        if self.path.exists():
            for raw in self.path.read_text().splitlines():
                if '"cost-state"' in raw:
                    totals = dict(json.loads(raw).get("modelUsage") or {})
        return totals

    def add_usage(self, usage: dict[str, Any] | None, *, absolute: bool = False) -> None:
        for model, values in (usage or {}).items():
            if absolute:
                self.totals[model] = dict(values)
                continue
            row = self.totals.setdefault(model, {})
            for key, value in values.items():
                if key != "contextWindow" and isinstance(value, int | float):
                    row[key] = row.get(key, 0) + value

    # ---- the inbox socket ----------------------------------------------------------------------

    def listen(self) -> None:
        if not self.socket_path or self.scenario.get("bg_no_socket"):
            return
        with contextlib.suppress(OSError):
            Path(self.socket_path).unlink()
        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        server.bind(self.socket_path)
        server.listen(8)
        self.server = server
        threading.Thread(target=self._accept, daemon=True).start()

    def _accept(self) -> None:
        assert self.server is not None
        while True:
            try:
                connection, _ = self.server.accept()
            except OSError:
                return
            with connection:
                data = b""
                while chunk := connection.recv(65536):
                    data += chunk
            for raw in data.split(b"\n"):
                if raw.strip():
                    self._arrived(json.loads(raw))

    def inbound(self, text: str) -> str:
        """What becomes of a line of the socket: ``accept``, ``hold``, ``review`` or ``refuse``.

        The rule of the pinned CLI: a value of ``crossSessionInbound`` decides (``hold``
        keeps the line without asking anyone); without one, the classes of both sessions do,
        and a line they do not let through waits for a person's approval (``review``). The
        class a sender states is read from the envelope around the text; a line without one
        states none.
        """
        if self.scenario.get("bg_hold_inbound"):
            return "hold"
        if self.inbound_setting in ("accept", "hold", "refuse"):
            return str(self.inbound_setting)
        found = ENVELOPE.match(text)
        mine = "bypass" if self.bypasses else "prompting"
        if found is not None:
            return "accept" if found.group(1) == mine else "review"
        return "review" if self.bypasses else "accept"

    def _arrived(self, frame: dict[str, Any]) -> None:
        text = str(frame["message"]["content"])
        verdict = self.inbound(text)
        self.fake.record("bg_socket", uuid=frame.get("uuid"), text=text, verdict=verdict)
        if verdict == "refuse":
            return
        if verdict in ("hold", "review"):
            # The message is noted, not delivered. One the rule per message kept waits for
            # an approval nobody gives, and the session shows as waiting from then on.
            self.record("system", subtype="informational", content="Held peer message")
            if verdict == "review":
                self.held += 1
                self.status(*(self._status or ("idle", None)))
            return
        with self.arrived:
            self.plain("queue-operation", operation="enqueue", content=text)
            self.inbox.append({"uuid": frame.get("uuid"), "text": text})
            self.arrived.notify_all()

    def absorb(self) -> None:
        """After a tool call: the lines queued meanwhile join the running turn."""
        with self.arrived:
            taken, self.inbox = self.inbox, []
        for line in taken:
            self.plain(
                "queue-operation",
                operation="remove",
                content=line["text"],
                reason="absorbed_mid_turn",
                commandUuid=line["uuid"],
                deliveryId=str(uuid.uuid4()),
            )
            self.record(
                "attachment",
                attachment={
                    "type": "queued_command",
                    "prompt": line["text"],
                    "source_uuid": line["uuid"],
                    "commandMode": "prompt",
                    "origin": {"kind": "peer", "from": "unknown"},
                    "isMeta": True,
                },
            )

    def next_line(self, timeout: float) -> dict[str, Any] | None:
        with self.arrived:
            self.arrived.wait_for(lambda: bool(self.inbox), timeout)
            return self.inbox.pop(0) if self.inbox else None

    # ---- turns ---------------------------------------------------------------------------------

    def gate(self, key: str) -> None:
        self.fake.gate(key)
        self.check_stop()

    def pid_gate(self) -> None:
        """A session whose entry has no process yet: wait for the gate ``bg_pid_gate``.

        The supervisor of the pinned CLI starts no process for an entry that was removed, so
        this process ends when its entry goes while it waits here: it never became a session.
        """
        path = self.scenario.get("bg_pid_gate")
        if not path:
            return
        self.fake.record("gate_wait", gate="bg_pid_gate")
        deadline = time.monotonic() + self.fake.patience
        while not Path(path).exists() and time.monotonic() < deadline:
            with state() as doc:
                if self.short not in doc["sessions"]:
                    os._exit(0)
            if self.fake.stopping.wait(0.02):
                return

    def check_stop(self) -> None:
        if self.stopped.is_set():
            if self.turn_open:
                # Like the pinned CLI: the totals a stop writes hold the requests of the
                # interrupted turn (the usage of the print side's interrupted result).
                self.add_usage(
                    {self.model: {"inputTokens": 50, "outputTokens": 10, "costUSD": 0.0005}}
                )
            self.end("stop")

    def play(self, line: dict[str, Any]) -> bool:
        """Write one line of a stream fixture as transcript records; False for a line without."""
        if line.get("type") == "assistant":
            self.request()
            message = dict(line["message"])
            message.setdefault("type", "message")
            self.assistant(message)
            return True
        if line.get("type") == "user":
            self.record("user", message=line["message"], toolUseResult={"stdout": ""})
            self.absorb()
            return True
        return False  # task, progress and retry frames have no transcript record

    def first_turn(self) -> None:  # noqa: PLR0912 - one branch per scripted behaviour
        scenario = self.scenario
        self.user(self.spec["prompt"], {"kind": "human"})
        waiting = scenario.get("bg_waiting")
        if waiting:
            self.status("waiting", str(waiting))
            self.stopped.wait(4 * self.fake.patience)
            self.end("stop")
        error = scenario.get("api_error")
        result: dict[str, Any] | None = None
        last_text = ""
        played = 0
        for line in self.fake.fixture():
            if line.get("type") == "result":
                result = line
                continue
            if error and played >= int(scenario.get("api_error_after", 0)):
                break
            if not self.play(line):
                continue
            if line.get("type") == "assistant":
                texts = [b["text"] for b in line["message"]["content"] if b.get("type") == "text"]
                last_text = "\n".join(texts) if texts else last_text
            played += 1
            if played == scenario.get("gate_after_lines"):
                self.gate("turn_gate")
        if "gate_after_lines" not in scenario:
            self.gate("turn_gate")
        if scenario.get("bg_die_mid_turn"):
            self.fake.record("bg_player_end", reason="died")
            os._exit(1)
        self.job_running = bool(scenario.get("background_gate"))
        if self.job_running:
            self.fake.record("background_started")
        if error:
            self.api_error(*self.fake.error(str(error)))
            self.turn_end()
            return
        if scenario.get("hold"):
            self.stopped.wait(4 * self.fake.patience)
            self.end("stop")
        final = dict(result or {})
        final.update(scenario.get("result_override", {}))
        text = self.fake.expand(str(final.get("result") or ""))
        if text != last_text:
            self.text(text, ident="msg_final")
        # Like the result of a print turn, the fixture names the session's totals after it.
        self.add_usage(final.get("modelUsage"), absolute=True)
        self.turn_end()

    def followup_turn(self, line: dict[str, Any]) -> None:
        running = self.job_running
        self.followups += 1
        self.plain("queue-operation", operation="dequeue")
        self.status("busy")
        origin = {"kind": "peer", "from": "unknown"}
        self.user(PEER_LEAD + line["text"] + PEER_TAIL, origin, ident=line["uuid"])
        self.gate("followup_gate")
        if self.account_down():
            self.fake.record("account_refused", line=line["uuid"])
            self.api_error(*self.fake.error(str(self.scenario["background_api_error"])))
        elif self.scenario.get("api_error_on_followup"):
            self.api_error(*self.fake.error(str(self.scenario["api_error_on_followup"])))
        else:
            if self.scenario.get("background_gate"):
                state_text = "still running" if running else "finished"
                self.text(f"Status: the background job is {state_text}.")
            else:
                self.text(f"Follow-up {self.followups} handled.")
            self.add_usage({self.model: {"inputTokens": 1000, "costUSD": 0.01}})
        self.turn_end()

    def notification_turn(self, text: str | None) -> None:
        """A turn the session starts by itself for the notification of its background job."""
        notice = "<task-notification>\n<status>completed</status>\n</task-notification>"
        self.plain("queue-operation", operation="enqueue", content=notice)
        self.plain("queue-operation", operation="dequeue")
        self.status("busy")
        self.user(notice, {"kind": "task-notification", "producer": "session-task"})
        if text is None or self.account_down():
            if text is not None:
                self.fake.record("account_refused", line="")
            self.api_error(*self.fake.error(str(self.scenario["background_api_error"])))
        else:
            self.text(text)
        self.turn_end()

    def typed_turn(self) -> None:
        """A person attached to the session and typed: a user message the runner never sent."""
        self.status("busy")
        self.fake.record("bg_typed")
        self.user("Typed into the session by a person at the host", {"kind": "human"})
        self.text("Answer to what was typed into the session.")
        self.turn_end()

    def account_down(self) -> bool:
        gate = self.scenario.get("account_gate")
        return bool(gate) and not Path(str(gate)).exists()

    def run(self) -> None:
        scenario = self.scenario
        signal.signal(signal.SIGUSR1, self.on_stop)
        if scenario.get("bg_ignore_stop"):
            signal.signal(signal.SIGUSR1, signal.SIG_IGN)
        signal.signal(
            signal.SIGTERM, signal.SIG_IGN if scenario.get("ignore_sigterm") else self.on_stop
        )
        self.pid_gate()
        self.status("busy")
        self.fake.record(
            "bg_player",
            short=self.short,
            session_id=self.session_id,
            pgid=os.getpgid(0),
            env=dict(os.environ),
            cwd=os.getcwd(),
            resumed=self.spec["resumed"],
        )
        self.listen()
        self.preamble()
        self.gate("bg_prompt_gate")  # a session that has not taken the prompt yet
        self.first_turn()
        job = scenario.get("background_gate")
        error_gate = scenario.get("background_error_gate")
        replace_gate = scenario.get("bg_replace_gate")
        typed_gate = scenario.get("bg_typed_gate")
        deadline = time.monotonic() + 4 * self.fake.patience
        while time.monotonic() < deadline:
            self.status("busy" if self.job_running else "idle")
            self.check_stop()
            if self.job_running and Path(str(job)).exists():
                self.job_running = False
                self.fake.record("background_done")
                self.notification_turn(str(scenario.get("background_text", "The suite passed.")))
                continue
            if error_gate and Path(str(error_gate)).exists():
                error_gate = None
                self.fake.record("background_turn_failed")
                self.notification_turn(None)
                continue
            if replace_gate and Path(str(replace_gate)).exists():
                replace_gate = None
                self.replace()
                continue
            if typed_gate and Path(str(typed_gate)).exists():
                typed_gate = None
                self.typed_turn()
                continue
            line = self.next_line(0.01)
            if line is not None:
                self.followup_turn(line)
        self.end("orphaned")

    def replace(self) -> None:
        """The session list names another process for the session (a supervisor restart)."""
        other = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(600)"])
        threading.Thread(target=other.wait, daemon=True).start()  # reaped when it is stopped
        self.fake.record("bg_replaced", other=other.pid)
        with state() as doc:
            entry = doc["sessions"].get(self.short)
            if entry is not None:
                entry["pid"] = other.pid
        self.stopped.wait(4 * self.fake.patience)
        self.end("stop", keep_entry=True)

    def on_stop(self, *_: Any) -> None:
        self.stopped.set()
        self.fake.stopping.set()

    def end(self, reason: str, *, keep_entry: bool = False) -> None:
        """The end of the process: the session totals, then the entry without a process."""
        with self.lock:
            if not self.scenario.get("bg_no_cost_state"):
                total = sum(float(v.get("costUSD", 0)) for v in self.totals.values())
                for _ in range(2):  # the pinned CLI writes the record twice at its end
                    self.plain("cost-state", totalCostUSD=total, modelUsage=self.totals)
            if self.server is not None:
                self.server.close()
                with contextlib.suppress(OSError):
                    Path(str(self.socket_path)).unlink()
            if not keep_entry:
                with state() as doc:
                    entry = doc["sessions"].get(self.short)
                    if entry is not None and entry.get("pid") == os.getpid():
                        for key in ("pid", "status", "waitingFor"):
                            entry.pop(key, None)
                        entry["state"] = "stopped"
            self.fake.record("bg_player_end", reason=reason)
            os._exit(0)


def player(fake: Any, short: str) -> int:
    Player(fake, short).run()
    return 0


def _capture(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        return []
    return [json.loads(x) for x in path.read_text().splitlines() if x.strip()]
