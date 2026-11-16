# SPDX-License-Identifier: Apache-2.0
"""The session module of background mode (09 18.2 "Background mode", 18.6).

The launch command and its settings file against fixed lists, and every output of the pinned
CLI the mode rests on against its recording (``tests/fixtures/recording/background_<version>``,
written by ``tests/real_cli/record.py``): the launch line and the refusals, the session list in
each state, ``daemon status``. Then the supervisor rule, the one reader of the session list, and
the handle of a session process with a real process in its place.
"""

from __future__ import annotations

import asyncio
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.adoption import read_identity
from claude_code_runner.bgpaths import SOCKET_PATH_MAX, bg_files, socket_path, socket_path_fits
from claude_code_runner.bgsession import (
    BgSession,
    SessionEntry,
    SessionList,
    Supervisor,
    background_argv,
    background_class_flags,
    load_supervisor,
    parse_launch,
    parse_sessions,
    parse_supervisor,
    prompt_text,
    send_frames,
    session_name,
    settings_document,
    socket_frame,
    store_supervisor,
    supervisor_verdict,
)
from claude_code_runner.commandline import ArgumentError
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.prompt import user_line

pytestmark = pytest.mark.anyio

RECORDED = (
    Path(__file__).resolve().parents[1]
    / "fixtures"
    / "recording"
    / f"background_{PINNED_CLAUDE_CODE_VERSION}"
)


def recorded(name: str) -> dict[str, Any]:
    loaded: dict[str, Any] = json.loads((RECORDED / f"{name}.json").read_text("utf-8"))
    return loaded


# ---- the launch command (18.2) ----------------------------------------------------------------


def argv(tool_class: str, tools: list[str], **more: Any) -> list[str]:
    values: dict[str, Any] = {
        "claude_bin": "/bin/claude",
        "model": "model-large",
        "resume": None,
        "name": "bothy-run",
        "tool_class": tool_class,
        "tools": tools,
        "add_dir": None,
        "system_prompt_file": Path("/s/prompt.md"),
        "socket_file": Path("/s/bg/x.sock"),
        "settings_file": Path("/s/bg/run.settings.json"),
    }
    return background_argv(**{**values, **more})


TAIL = [
    "--append-system-prompt-file",
    "/s/prompt.md",
    "--messaging-socket-path",
    "/s/bg/x.sock",
    "--settings",
    "/s/bg/run.settings.json",
]
HEAD = ["/bin/claude", "--bg", "--name", "bothy-run", "--model", "model-large"]


def test_the_argument_list_of_each_tool_class() -> None:
    assert argv("chat", []) == [
        *HEAD,
        "--permission-mode",
        "dontAsk",
        "--tools",
        "",
        "--disallowedTools",
        "mcp__*",
        *TAIL,
    ]
    assert argv("scoped", ["Read", "Bash(git status)"]) == [
        *HEAD,
        "--permission-mode",
        "dontAsk",
        "--allowedTools",
        "Read",
        "Bash(git status)",
        "--disallowedTools",
        "AskUserQuestion",
        *TAIL,
    ]
    assert argv("scoped", []) == [
        *HEAD,
        "--permission-mode",
        "dontAsk",
        "--disallowedTools",
        "AskUserQuestion",
        *TAIL,
    ]
    assert argv("host", []) == [
        *HEAD,
        "--permission-mode",
        "bypassPermissions",
        "--disallowedTools",
        "AskUserQuestion",
        *TAIL,
    ]


def test_a_resume_and_the_incoming_folder_have_their_places() -> None:
    resumed = argv("host", [], resume="0192d3a0-0000-4000-8000-0000000000aa", add_dir=Path("/f"))
    assert resumed[:4] == [
        "/bin/claude",
        "--resume",
        "0192d3a0-0000-4000-8000-0000000000aa",
        "--bg",
    ]
    at = resumed.index("--add-dir")
    assert resumed[at : at + 3] == ["--add-dir", "/f", "--append-system-prompt-file"]


def test_no_flag_of_print_mode_and_no_session_id_of_the_runner() -> None:
    for tool_class in ("chat", "scoped", "host"):
        flags = argv(tool_class, ["Read"])
        for absent in ("-p", "--permission-prompts", "--session-id", "--input-format"):
            assert absent not in flags


def test_values_that_must_not_become_arguments_are_refused() -> None:
    with pytest.raises(ArgumentError):
        argv("scoped", [], model="-x")
    with pytest.raises(ArgumentError):
        argv("chat", [], add_dir=Path("/f"))
    with pytest.raises(ArgumentError):
        background_class_flags("scoped", ["--dangerous"])
    with pytest.raises(ArgumentError):
        background_class_flags("other", [])


def test_the_settings_file_holds_exactly_three_members() -> None:
    env = {"HOME": "/home/alex", "PATH": "/usr/bin", "TMPDIR": "/state/tmp/run", "SECRET": "x"}
    assert settings_document(env) == {
        "crossSessionInbound": "accept",
        "worktree": {"bgIsolation": "none"},
        "env": {"CLAUDE_PRODUCT_RUN": "1", "DISABLE_AUTOUPDATER": "1", "TMPDIR": "/state/tmp/run"},
    }
    forwarded = settings_document({**env, "SSH_AUTH_SOCK": "/tmp/agent.sock"})
    assert forwarded["env"]["SSH_AUTH_SOCK"] == "/tmp/agent.sock"
    assert set(forwarded["env"]) == {
        "CLAUDE_PRODUCT_RUN",
        "DISABLE_AUTOUPDATER",
        "TMPDIR",
        "SSH_AUTH_SOCK",
    }


def test_the_prompt_is_the_context_then_the_trigger_part() -> None:
    line, _ = user_line(["Context with marks", "/compact please"])
    assert prompt_text(line) == "Context with marks\n\n/compact please"
    assert prompt_text("not json") is None
    assert prompt_text(json.dumps({"uuid": "u", "message": {"content": 7}})) is None


def test_an_inbox_line_keeps_the_uuid_and_carries_the_text() -> None:
    line, line_uuid = user_line(["Follow-up from Sam (@sam):\n\nThe café footer, 日本語, €"])
    frame = socket_frame(line)
    assert frame is not None
    assert frame.endswith(b"\n")
    assert json.loads(frame) == {
        "type": "user",
        "uuid": line_uuid,
        "message": {
            "role": "user",
            "content": "Follow-up from Sam (@sam):\n\nThe café footer, 日本語, €",
        },
    }
    assert socket_frame(b"{}") is None
    assert socket_frame(json.dumps({"uuid": 5, "message": {"content": "x"}})) is None


def test_the_session_name_carries_the_runner_s_prefix() -> None:
    assert session_name("0192d3a0-0000-7000-8000-000000000001") == (
        "bothy-0192d3a0-0000-7000-8000-000000000001"
    )


# ---- what the launch command prints -----------------------------------------------------------


def launch_of(case: str) -> Any:
    out = recorded("launch")[case]
    return parse_launch(out["exit"], out["stdout"], out["stderr"])


def test_the_launch_line_of_the_pinned_cli() -> None:
    new = launch_of("new")
    assert new.refusal is None
    assert new.notes == 0
    assert not new.exit_error
    assert new.short_id is not None
    assert len(new.short_id) == 8
    assert new.started_service is True  # no supervisor ran before
    again = launch_of("resumed_in_place")
    assert (again.short_id, again.notes, again.started_service) == (new.short_id, 0, False)


def test_a_copy_is_known_by_its_note_and_its_other_id() -> None:
    copy = launch_of("copy_while_running")
    assert copy.notes == 1
    assert copy.short_id is not None
    original = recorded("launch")["copy_while_running"]["stderr"].split()[2]
    assert copy.short_id != original


def test_the_refusals_of_the_pinned_cli() -> None:
    untrusted, bypass = launch_of("untrusted"), launch_of("bypass")
    assert (untrusted.short_id, untrusted.refusal, untrusted.exit_error) == (
        None,
        "untrusted",
        True,
    )
    assert (bypass.short_id, bypass.refusal, bypass.exit_error) == (None, "bypass", True)


def test_any_other_output_is_a_failed_launch() -> None:
    error = parse_launch(1, "", "error: agent view is switched off")
    assert (error.refusal, error.exit_error) == ("failed", True)
    silent = parse_launch(0, "", "")  # exit 0 without the line: not certain that nothing started
    assert (silent.short_id, silent.refusal, silent.exit_error) == (None, "failed", False)
    late = parse_launch(None, "", "")  # the command did not end in time
    assert (late.refusal, late.exit_error) == ("failed", False)
    assert parse_launch(1, "backgrounded · 1a2b3c4d · x", "").short_id is None


# ---- the session list (18.4) --------------------------------------------------------------------


def entry_of(state: str) -> SessionEntry:
    [entry] = parse_sessions(json.dumps([recorded("sessions")[state]])) or []
    return entry


def test_the_entries_of_the_pinned_cli_in_each_state() -> None:
    busy, idle = entry_of("busy"), entry_of("idle")
    assert busy.own
    assert busy.has_process
    assert busy.pid is not None
    assert (busy.status, idle.status) == ("busy", "idle")
    assert busy.session_id.startswith(busy.short_id)
    assert busy.kind == "background"
    assert entry_of("busy_between_turns").status == "busy"
    waiting = entry_of("waiting")
    assert (waiting.status, waiting.waiting_for) == ("waiting", "input needed")
    for gone in (entry_of("stopped"), entry_of("failed")):
        assert (gone.has_process, gone.pid, gone.status) == (False, None, None)


def test_state_is_never_read() -> None:
    assert "state" not in SessionEntry.__dataclass_fields__
    raw = dict(recorded("sessions")["idle"])
    seen = {parse_sessions(json.dumps([{**raw, "state": s}]))[0] for s in ("done", "working", 7)}  # type: ignore[index]
    assert len(seen) == 1


def test_of_another_program_s_entry_only_its_place_is_kept() -> None:
    raw = {**recorded("sessions")["waiting"], "name": "my-own-session", "kind": "interactive"}
    [entry] = parse_sessions(json.dumps([raw])) or []
    assert entry == SessionEntry(
        short_id=raw["id"],
        session_id=raw["sessionId"],
        name="my-own-session",
        own=False,
        has_process=True,
    )


def test_an_unreadable_list_is_no_list() -> None:
    assert parse_sessions("not json") is None
    assert parse_sessions('{"id": "x"}') is None
    assert parse_sessions("[]") == []
    assert parse_sessions('[7, {"name": "bothy-x", "pid": true}]') == [
        SessionEntry("", "", "bothy-x", own=True, has_process=True, pid=None)
    ]


# ---- the supervisor (18.2) ----------------------------------------------------------------------


def test_daemon_status_of_the_pinned_cli() -> None:
    commands = recorded("commands")
    none = commands["daemon_status_not_running"]
    assert parse_supervisor(none["exit"], none["stdout"]) == (True, None)
    runs = commands["daemon_status_running"]
    readable, found = parse_supervisor(runs["exit"], runs["stdout"])
    assert readable
    assert found is not None
    assert found.version == PINNED_CLAUDE_CODE_VERSION
    assert found.pid is not None
    assert parse_supervisor(None, "") == (False, None)
    assert parse_supervisor(0, "something else") == (False, None)


def test_only_the_runner_s_own_supervisor_in_the_pinned_version_is_used() -> None:
    pin = PINNED_CLAUDE_CODE_VERSION
    ours = {"pid": 4711, "lstart": "Mon Nov 16 10:00:00 2026"}

    def lstart(pid: int) -> str | None:
        return {4711: "Mon Nov 16 10:00:00 2026", 4712: "Mon Nov 16 11:00:00 2026"}.get(pid)

    assert supervisor_verdict(None, None, pin, lstart) is None  # the launch starts one
    assert supervisor_verdict(Supervisor(4711, pin), ours, pin, lstart) is None
    foreign = "background_supervisor_foreign"
    assert supervisor_verdict(Supervisor(4712, pin), ours, pin, lstart) == foreign
    assert supervisor_verdict(Supervisor(4711, pin), None, pin, lstart) == foreign
    assert supervisor_verdict(Supervisor(None, pin), ours, pin, lstart) == foreign
    reused = {"pid": 4712, "lstart": "Mon Nov 16 10:00:00 2026"}  # the process id was reused
    assert supervisor_verdict(Supervisor(4712, pin), reused, pin, lstart) == foreign
    assert (
        supervisor_verdict(Supervisor(4711, "2.1.999"), ours, pin, lstart)
        == "background_supervisor_version"
    )


def test_the_recorded_supervisor_is_state_of_the_runner(tmp_path: Path) -> None:
    assert load_supervisor(tmp_path) is None
    store_supervisor(tmp_path, 4711, "Mon Nov 16 10:00:00 2026")
    assert load_supervisor(tmp_path) == {"pid": 4711, "lstart": "Mon Nov 16 10:00:00 2026"}
    path = tmp_path / "bg" / "supervisor.json"
    assert oct(path.stat().st_mode & 0o777) == "0o600"
    path.write_text('{"pid": 1, "lstart": "x"}')
    assert load_supervisor(tmp_path) is None  # no process id a record may name
    path.write_text("[]")
    assert load_supervisor(tmp_path) is None


# ---- the files of a turn ------------------------------------------------------------------------


def test_the_socket_path_is_short_and_bounded() -> None:
    run = "0192d3a0-0000-7000-8000-000000000001"
    path = socket_path(Path("/var/lib/runner"), run)
    assert path.parent == Path("/var/lib/runner/bg")
    assert len(path.name) == 21
    assert path != socket_path(Path("/var/lib/runner"), run.replace("1", "2"))
    assert bg_files(Path("/s"), run).socket == socket_path(Path("/s"), run)
    fits = "/" + "x" * (SOCKET_PATH_MAX - len("/bg/0123456789abcdef.sock") - 1)
    assert socket_path_fits(Path(fits))
    assert not socket_path_fits(Path(fits + "x"))


@pytest.fixture
def short_dir() -> Iterator[Path]:
    base = os.environ.get("CLAUDE_CODE_RUNNER_SHORT_TMP") or "/tmp"
    path = Path(tempfile.mkdtemp(prefix="ccr", dir=base))
    try:
        yield path
    finally:
        shutil.rmtree(path, ignore_errors=True)


def test_lines_reach_a_listening_socket_and_a_missing_one_takes_none(short_dir: Path) -> None:
    path = short_dir / "inbox.sock"
    assert send_frames(path, [b"{}\n"], 1.0) is False
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as server:
        server.bind(str(path))
        server.listen(1)
        assert send_frames(path, [b'{"a":1}\n', b'{"b":2}\n'], 1.0) is True
        connection, _ = server.accept()
        with connection:
            assert connection.recv(100) == b'{"a":1}\n{"b":2}\n'


# ---- the one reader of the session list ---------------------------------------------------------


class FakeCli:
    """The control commands with scripted answers and a count of the list calls."""

    def __init__(self) -> None:
        self.lists = 0
        self.entries: list[SessionEntry] | None = []
        self.stopped: list[str] = []
        self.removed: list[str] = []
        self.on_stop: Any = None

    async def sessions(self) -> list[SessionEntry] | None:
        self.lists += 1
        await asyncio.sleep(0)
        return self.entries

    async def stop(self, short_id: str) -> bool:
        self.stopped.append(short_id)
        if self.on_stop is not None:
            self.on_stop()
        return True

    async def remove(self, short_id: str) -> bool:
        self.removed.append(short_id)
        return True


async def test_a_reading_is_shared_until_it_is_too_old() -> None:
    cli, now = FakeCli(), [100.0]
    sessions = SessionList(cli, 2.0, clock=lambda: now[0])  # type: ignore[arg-type]
    first = await sessions.read()
    assert first is not None
    assert cli.lists == 1
    now[0] = 101.9
    assert await sessions.read() is first
    assert cli.lists == 1
    now[0] = 102.0
    second = await sessions.read()
    assert second is not first
    assert cli.lists == 2
    # A turn end asks for a reading taken after it, however young the last one is.
    now[0] = 102.5
    third = await sessions.read(after=102.5)
    assert third is not second
    assert third is not None
    assert third.at == 102.5
    assert await sessions.read(after=102.4) is third
    # Two turns that ask at once share one list command.
    now[0] = 110.0
    one, two = await asyncio.gather(sessions.read(), sessions.read())
    assert one is two
    assert cli.lists == 4


async def test_an_unreadable_list_gives_no_reading_and_keeps_the_last() -> None:
    cli = FakeCli()
    sessions = SessionList(cli, 0.0)  # type: ignore[arg-type]
    first = await sessions.read()
    cli.entries = None
    assert await sessions.read(after=sessions.now()) is None
    assert sessions.last is first


async def test_a_reading_finds_own_entries_and_every_entry_of_a_session() -> None:
    cli = FakeCli()
    mine = SessionEntry("aaaaaaaa", "aaaaaaaa-1", "bothy-run", own=True, has_process=True, pid=7)
    theirs = SessionEntry("aaaaaaaa", "aaaaaaaa-1", "other", own=False, has_process=False)
    cli.entries = [theirs, mine]
    reading = await SessionList(cli, 1.0).read()  # type: ignore[arg-type]
    assert reading is not None
    assert reading.entry("aaaaaaaa") is mine
    assert reading.entry("bbbbbbbb") is None
    assert reading.named("other") is theirs
    assert reading.named("nobody") is None
    assert reading.of_session("aaaaaaaa-1") == [theirs, mine]


# ---- one session as a turn process --------------------------------------------------------------


@pytest.fixture
def process() -> Iterator[subprocess.Popen[bytes]]:
    """A process in a group of its own, as a session process is."""
    code = "import time; time.sleep(600)"
    proc = subprocess.Popen([sys.executable, "-c", code], start_new_session=True)
    try:
        yield proc
    finally:
        if proc.poll() is None:
            proc.kill()
        proc.wait()


def session_of(proc: subprocess.Popen[bytes], cli: FakeCli, **more: Any) -> BgSession:
    identity = read_identity(proc.pid, 10.0)
    assert identity is not None
    values: dict[str, Any] = {
        "short_id": "1a2b3c4d",
        "session_id": "1a2b3c4d-0000-4000-8000-000000000000",
        "pid": proc.pid,
        "lstart": identity.lstart,
        "socket_file": Path("/nonexistent/inbox.sock"),
        "timeout": 10.0,
    }
    return BgSession(cli, **{**values, **more})  # type: ignore[arg-type]


async def test_a_session_is_known_by_its_process_id_and_start_time(
    process: subprocess.Popen[bytes],
) -> None:
    cli = FakeCli()
    session = session_of(process, cli)
    assert session.alive()
    assert session.matches()
    assert session.may_signal()
    assert session.poll() is None
    assert session.exit_label() == "running"
    other = session_of(process, cli, lstart="Thu Jan  1 00:00:00 1970")
    assert not other.alive()
    assert not other.may_signal()
    assert other.exit_label() == "unknown"


async def test_a_stop_is_claude_stop_and_returns_once_the_process_is_gone(
    process: subprocess.Popen[bytes],
) -> None:
    cli = FakeCli()
    cli.on_stop = lambda: (process.send_signal(signal.SIGKILL), process.wait())
    session = session_of(process, cli)
    assert session.alive()
    await session.stop(5.0, 5.0, 0.01, {"run_id": "r"})
    assert cli.stopped == ["1a2b3c4d"]
    assert not session.alive()
    assert session.poll() == 0
    assert session.exit_label() == "0"  # a stop of the runner, no escalation
    assert await session.write_bytes(b"{}\n") is False  # a stopped session takes no input
    await session.stop(5.0, 5.0, 0.01)  # a second stop changes nothing
    assert cli.stopped == ["1a2b3c4d"]
    assert await session.remove() is True
    assert cli.removed == ["1a2b3c4d"]
    assert await session.wait(1.0) is True


async def test_a_stop_escalates_to_the_group_when_the_cli_leaves_the_process(
    process: subprocess.Popen[bytes], caplog: pytest.LogCaptureFixture
) -> None:
    cli = FakeCli()  # ``claude stop`` answers, but the process lives on
    session = session_of(process, cli)
    waiter = asyncio.ensure_future(asyncio.to_thread(process.wait))
    with caplog.at_level("WARNING", logger="claude_code_runner"):
        await session.stop(0.05, 5.0, 0.01, {"run_id": "r"})
    await waiter
    assert process.returncode == -signal.SIGTERM
    escalated = [json.loads(r.getMessage()) for r in caplog.records]
    assert [(e["event"], e["signal"]) for e in escalated] == [("stop_escalated", "SIGTERM")]
    assert session.exit_label() == "SIGTERM"


async def test_inputs_go_to_the_inbox_socket_as_lines(
    process: subprocess.Popen[bytes], short_dir: Path
) -> None:
    path = short_dir / "inbox.sock"
    session = session_of(process, FakeCli(), socket_file=path)
    line, line_uuid = user_line(["Follow-up from Sam (@sam):\n\nOne more"])
    assert await session.write_bytes(line.encode() + b"\n") is False  # nobody listens
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as server:
        server.bind(str(path))
        server.listen(1)
        assert await session.write_bytes(line.encode() + b"\n") is True
        connection, _ = server.accept()
        with connection:
            got = json.loads(connection.recv(4096))
    assert (got["uuid"], got["message"]["content"]) == (
        line_uuid,
        "Follow-up from Sam (@sam):\n\nOne more",
    )
    assert await session.write_bytes(b"not a line\n") is False
    session.close_stdin()  # nothing to close


async def test_a_session_without_an_id_is_neither_stopped_nor_removed_by_name(
    process: subprocess.Popen[bytes],
) -> None:
    cli = FakeCli()
    session = session_of(process, cli, short_id="", lstart="Thu Jan  1 00:00:00 1970")
    await session.stop(0.05, 0.05, 0.01)
    assert await session.remove() is False
    assert cli.stopped == []
    assert cli.removed == []
