# SPDX-License-Identifier: Apache-2.0
"""Hosted turns an earlier runner left behind (docs/spec/26-builtin-agents.md 3.2 item 5).

Hosted turns are not adopted. At its start the runner stops a process that still matches its
turn record under ``hosted/<agent_id>/turns/`` and removes the record; a process that does not
match is never signalled, and its record goes only when its locks are free. The stray process of
these tests is no child of the test, like a turn whose runner died.

A record is a file, so nothing in it is taken as given
(docs/decisions.d/FIX-RUNNER-HOSTING-REVIEW.md): a record that is not what the runner writes
leads to no signal and no lock, a process is signalled only when its command line names the
record's session and the control folder of the agent, and one record that cannot be handled never
stops the start for the others.
"""

from __future__ import annotations

import contextlib
import json
import os
import signal
import subprocess
import sys
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.testing._patience import PATIENCE, eventually
from scenarios import conftest as scenarios
from scenarios.conftest import LogLines

from claude_code_runner import adoption
from claude_code_runner.adoption import ProcessIdentity, read_identity
from claude_code_runner.hosting import stop_leftover_turns
from claude_code_runner.locks import acquire, release
from claude_code_runner.statedir import ensure_private_dir
from claude_code_runner.turn import write_turn_record

pytestmark = pytest.mark.anyio

logs = scenarios.logs

AGENT = "0192f4a0-7b1c-7d07-84b0-358556a08a99"
# The folders are read in the order of their names: this agent comes after `AGENT`.
OTHER_AGENT = "0192f4a0-7b1c-7d07-84b0-358556a08aff"
RUN = "0192f4a0-7b1c-7d07-84b0-000000000001"
THREAD = "0192f4a0-7b1c-7d07-84b0-000000000002"
REF = "0192d3a0-0000-4000-8000-0000000000dd"
_GRACE: dict[str, float] = {"stop_grace": 1.0, "kill_grace": 1.0, "poll": 0.01, "timeout": 30.0}

Strays = Callable[..., int]


def _control(agent: str = AGENT) -> str:
    """The settings file in the control folder of a hosted agent (26 5.3, ``--settings``)."""
    return f"/srv/agents/{agent}.control/settings.json"


def _hosted_turn(agent: str = AGENT, ref: str = REF) -> list[str]:
    """The arguments that make a process a turn of a hosted agent: its session and settings."""
    return ["--session-id", ref, "--settings", _control(agent)]


def _events(logs: LogLines, event: str) -> list[dict[str, str]]:
    out = []
    for record in logs.records:
        try:
            line = json.loads(record.getMessage())
        except ValueError:
            continue
        if isinstance(line, dict) and line.get("event") == event:
            out.append(line)
    return out


def _alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def _tree(root: Path) -> list[tuple[str, bool]]:
    """Every entry under ``root``: a lock file or folder the runner made would show here."""
    return sorted((str(p.relative_to(root)), p.is_dir()) for p in root.rglob("*"))


@pytest.fixture
async def strays(tmp_path: Path) -> AsyncIterator[Strays]:
    """Starts processes like a turn that outlived its runner: each in a group of its own and no
    child of this test (so it is reaped once it ends), with the given arguments in its command
    line. SIGINT ends such a process, so a signal the runner sent shows as its end."""
    started: list[int] = []
    turn = (
        "import pathlib, signal, sys, time\n"
        "signal.signal(signal.SIGINT, signal.SIG_DFL)\n"
        "pathlib.Path(sys.argv[1]).touch()\n"
        "time.sleep(3600)\n"  # only a bound for an orphan: the fixture ends the process
    )
    starter = (
        "import subprocess, sys\n"
        "gone = subprocess.DEVNULL\n"
        "turn = subprocess.Popen([sys.executable, '-c', *sys.argv[1:]], start_new_session=True,\n"
        "                        stdin=gone, stdout=gone, stderr=gone)\n"
        "print(turn.pid)\n"
    )

    async def start(*arguments: str) -> int:
        ready = tmp_path / f"ready-{len(started)}"
        out = subprocess.run(
            [sys.executable, "-c", starter, turn, str(ready), *arguments],
            capture_output=True,
            text=True,
            check=True,
        )
        pid = int(out.stdout)
        started.append(pid)
        await eventually(ready.exists, "the stray process to run")
        return pid

    try:
        yield start
    finally:
        for pid in started:
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(pid, signal.SIGKILL)


@pytest.fixture
async def stray(strays: Strays) -> int:
    """A running turn of the hosted agent `AGENT` with the session `REF`."""
    return await strays(*_hosted_turn())


def _record(pid: int, **changes: Any) -> dict[str, Any]:
    """What a runner wrote about a hosted run whose turn is the process ``pid``."""
    identity = read_identity(pid, 30.0)
    assert identity is not None
    return {
        "run_id": RUN,
        "thread_id": THREAD,
        "session_ref": REF,
        "pid": pid,
        "pgid": pid,
        "identity": {"lstart": identity.lstart, "command": identity.command},
        **changes,
    }


def _leftover(state: Path, pid: int, *, agent: str = AGENT, **changes: Any) -> Path:
    """The record a crashed runner left of a hosted run whose turn is the process ``pid``,
    written as the runner writes it (private folders, `write_turn_record`)."""
    path = state / "hosted" / agent / "turns" / f"{RUN}.json"
    write_turn_record(path, _record(pid, **changes))
    return path


def _forged(state: Path, content: object, *, name: str = RUN) -> Path:
    """A file in the place of a record that no runner wrote."""
    turns = ensure_private_dir(state / "hosted" / AGENT / "turns")
    path = turns / f"{name}.json"
    path.write_text(content if isinstance(content, str) else json.dumps(content))
    return path


async def test_a_leftover_hosted_turn_that_runs_is_stopped_and_its_record_removed(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    path = _leftover(tmp_path, stray)
    # SIGINT ends it: an escalation would come only after the whole patience.
    await stop_leftover_turns(tmp_path, **{**_GRACE, "stop_grace": PATIENCE})
    # The stop returns only once no process of the group is left. The group is asked, as the
    # stop asks it: on macOS the ended process itself still answers until it is reaped.
    assert not scenarios._group_alive(stray)
    assert not path.exists()
    assert _events(logs, "turn_adopted") == [
        {
            "event": "turn_adopted",
            "action": "stop",
            "agent_id": AGENT,
            "run_id": RUN,
            "thread_id": THREAD,
        }
    ]
    assert _events(logs, "stop_escalated") == []


async def test_the_record_of_a_gone_hosted_turn_is_removed_and_an_unreadable_one_kept(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    path = _leftover(tmp_path, stray)
    unreadable = path.with_name("0192f4a0-7b1c-7d07-84b0-000000000003.json")
    unreadable.write_text("{")
    os.killpg(stray, signal.SIGKILL)
    await eventually(lambda: not _alive(stray), "the end of the stray process")
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert not path.exists()
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]
    assert unreadable.exists()
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [unreadable.stem]


async def test_a_record_written_before_its_process_started_is_removed(
    tmp_path: Path, logs: LogLines
) -> None:
    # The runner writes the record first and the process IDs once the turn runs; a crash in
    # between leaves a record without them, which names no process.
    path = _forged(tmp_path, {"run_id": RUN, "thread_id": THREAD, "session_ref": REF})
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert not path.exists()
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]


async def test_a_process_that_does_not_match_its_hosted_record_is_never_signalled(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    # Any doubt about the identity counts as gone (09 18.9): no signal. A turn that lives on
    # holds the thread's lock through its inherited descriptor, so its record stays.
    identity = {"lstart": "Thu Jan  1 00:00:00 1970", "command": "claude"}
    path = _leftover(tmp_path, stray, identity=identity)
    lock = tmp_path / "hosted" / AGENT / "locks" / f"thread-{THREAD}.lock"
    held = await acquire(lock, 0, 0.01)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert path.exists()
    assert len(_events(logs, "adoption_lock_busy")) == 1
    assert _events(logs, "turn_adopted") == []
    release(held)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert not path.exists()
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]
    assert _alive(stray)


async def test_no_hosted_folder_is_no_leftover(tmp_path: Path) -> None:
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert list(tmp_path.iterdir()) == []


# ---- a record is a file: nothing in it is taken as given -------------------------------------


async def test_a_forged_record_never_signals_the_foreign_process_it_names(
    tmp_path: Path, strays: Strays, logs: LogLines
) -> None:
    # A process that is no turn: the record names it with its real group and start time and a
    # "session reference" of path characters that its command line happens to contain.
    marker = "x/../../escape/evil"
    foreign = await strays(marker)
    path = _forged(tmp_path, _record(foreign, session_ref=marker))
    before = _tree(tmp_path)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _alive(foreign)
    assert _tree(tmp_path) == before  # the record stays, no lock was taken anywhere
    assert _events(logs, "turn_adopted") == []
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [path.stem]


async def test_a_forged_record_takes_no_lock_outside_the_lock_folder(
    tmp_path: Path, strays: Strays, logs: LogLines
) -> None:
    # A foreign group: the process does not match, so the runner would take the record's locks.
    # A lock named after this "session reference" would lie outside the lock folder.
    foreign = await strays("no-turn")
    other_group = await strays("no-turn")
    path = _forged(tmp_path, _record(foreign, pgid=other_group, session_ref="x/../../escape/evil"))
    before = _tree(tmp_path)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _tree(tmp_path) == before
    assert not (tmp_path / "hosted" / AGENT / "escape").exists()
    assert _alive(foreign)
    assert _alive(other_group)
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [path.stem]


@pytest.mark.parametrize(
    "arguments",
    [
        [REF],  # the reference somewhere in the command line
        ["--session-id", REF],  # a session of Claude Code, of no hosted agent
        ["--session-id", f"{REF}-copy", "--settings", _control()],  # only a part of the argument
        ["--settings", _control(), "--resume-from", REF],  # the argument of another flag
        _hosted_turn(OTHER_AGENT),  # a turn of another hosted agent
    ],
    ids=["reference_only", "no_control_folder", "part_of_argument", "other_flag", "other_agent"],
)
async def test_a_process_that_is_no_turn_of_the_agent_is_never_signalled(
    tmp_path: Path, strays: Strays, logs: LogLines, arguments: list[str]
) -> None:
    # The record is well formed and names the process with its real group and start time, and
    # the reference stands in its command line. It still is no turn of this agent's session.
    foreign = await strays(*arguments)
    path = _leftover(tmp_path, foreign)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _alive(foreign)
    assert _events(logs, "stop_escalated") == []
    # It counts as gone: its record goes, because no turn holds the locks of the record.
    assert [e["action"] for e in _events(logs, "turn_adopted")] == ["leave"]
    assert not path.exists()


@pytest.mark.parametrize(
    "changes",
    [
        {"run_id": "0192f4a0-7b1c-7d07-84b0-0000000000ff"},  # not the run of its file name
        {"run_id": 5},
        {"thread_id": "../../escape"},
        {"thread_id": None},
        {"session_ref": None},
        {"session_ref": REF.replace("-", "")},  # a UUID, but not in its canonical text
        {"pid": 2**40},  # os.getpgid raises OverflowError for it
        {"pid": "4711"},
        {"pid": True},
        {"pid": 1.5},
        {"pid": 1},
        {"pid": -4711},
        {"pid": None},  # a group without a process
        {"pgid": 2**31},
        {"pgid": 0},
        {"pgid": None},  # a process without a group
    ],
    ids=[
        "run_of_another_file",
        "run_no_text",
        "thread_with_path",
        "thread_missing",
        "session_missing",
        "session_not_canonical",
        "pid_too_large",
        "pid_text",
        "pid_truth_value",
        "pid_fraction",
        "pid_init",
        "pid_negative",
        "pid_missing",
        "pgid_too_large",
        "pgid_zero",
        "pgid_missing",
    ],
)
async def test_a_record_that_no_runner_wrote_is_unreadable_and_the_start_goes_on(
    tmp_path: Path, strays: Strays, logs: LogLines, changes: dict[str, Any]
) -> None:
    # One member differs from what the runner writes; everything else names a running turn of
    # the agent. Nothing is signalled, no lock is taken, the record stays, and the leftover turn
    # of the next agent is still stopped.
    named = await strays(*_hosted_turn())
    path = _forged(tmp_path, {**_record(named), **changes})
    other = await strays(*_hosted_turn(OTHER_AGENT))
    other_path = _leftover(tmp_path, other, agent=OTHER_AGENT)
    await stop_leftover_turns(tmp_path, **{**_GRACE, "stop_grace": PATIENCE})
    assert _alive(named)
    assert path.exists()
    assert not (tmp_path / "hosted" / AGENT / "locks").exists()
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [path.stem]
    assert not scenarios._group_alive(other)
    assert not other_path.exists()
    assert [e["agent_id"] for e in _events(logs, "turn_adopted")] == [OTHER_AGENT]


@pytest.mark.parametrize(
    "content",
    ["[]", '"text"', "7", "null", "[" * 100_000],
    ids=["list", "text", "number", "null", "nested_too_deep"],
)
async def test_a_file_that_is_no_json_object_is_unreadable(
    tmp_path: Path, logs: LogLines, content: str
) -> None:
    path = _forged(tmp_path, content)
    before = _tree(tmp_path)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _tree(tmp_path) == before
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [path.stem]


async def test_a_link_or_a_pipe_in_the_place_of_a_record_is_unreadable(
    tmp_path: Path, stray: int, logs: LogLines
) -> None:
    # A link to a record elsewhere is not followed; a named pipe never holds up the start.
    real = tmp_path / "elsewhere.json"
    real.write_text(json.dumps(_record(stray)))
    turns = ensure_private_dir(tmp_path / "hosted" / AGENT / "turns")
    link = turns / f"{RUN}.json"
    link.symlink_to(real)
    pipe = turns / "0192f4a0-7b1c-7d07-84b0-000000000003.json"
    os.mkfifo(pipe)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _alive(stray)
    assert link.is_symlink()
    assert sorted(e["record"] for e in _events(logs, "turn_record_unreadable")) == sorted(
        [link.stem, pipe.stem]
    )


@pytest.mark.parametrize(
    "place", ["open_turns", "linked_turns", "linked_agent", "other_name", "no_agent"]
)
async def test_records_outside_a_private_folder_of_an_agent_are_unreadable(
    tmp_path: Path, stray: int, logs: LogLines, place: str
) -> None:
    # The runner keeps the records of a hosted agent in hosted/<agent_id>/turns, both folders
    # its own with mode 0700 (26 3.2 item 2). Records anywhere else are not its records.
    hosted = ensure_private_dir(tmp_path / "hosted")
    elsewhere = ensure_private_dir(tmp_path / "elsewhere")
    name = {"other_name": AGENT.upper(), "no_agent": "no-agent"}.get(place, AGENT)
    if place == "linked_agent":
        (hosted / name).symlink_to(elsewhere, target_is_directory=True)
    else:
        ensure_private_dir(hosted / name)
    turns = hosted / name / "turns"
    if place == "linked_turns":
        ensure_private_dir(elsewhere / "turns")
        turns.symlink_to(elsewhere / "turns", target_is_directory=True)
    else:
        ensure_private_dir(turns)
    if place == "open_turns":
        turns.chmod(0o755)
    path = turns / f"{RUN}.json"
    path.write_text(json.dumps(_record(stray)))
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert _alive(stray)
    assert path.exists()
    assert not (hosted / name / "locks").exists()
    assert [e["record"] for e in _events(logs, "turn_record_unreadable")] == [path.stem]


async def test_an_error_of_the_state_folder_at_one_record_does_not_stop_the_start(
    tmp_path: Path, strays: Strays, logs: LogLines
) -> None:
    # The lock folder of the first agent is a file: its gone turn cannot be settled. The start
    # goes on, and the leftover turn of the next agent is stopped.
    path = _forged(tmp_path, {"run_id": RUN, "thread_id": THREAD, "session_ref": REF})
    (tmp_path / "hosted" / AGENT / "locks").write_text("no folder")
    other = await strays(*_hosted_turn(OTHER_AGENT))
    other_path = _leftover(tmp_path, other, agent=OTHER_AGENT)
    await stop_leftover_turns(tmp_path, **{**_GRACE, "stop_grace": PATIENCE})
    assert path.exists()
    [line] = _events(logs, "turn_record_unreadable")
    assert (line["record"], line["error"]) == (path.stem, "StateDirError")
    assert not scenarios._group_alive(other)
    assert not other_path.exists()


async def test_an_unexpected_error_at_one_record_keeps_it_and_releases_its_locks(
    tmp_path: Path, logs: LogLines, monkeypatch: pytest.MonkeyPatch
) -> None:
    first = _forged(tmp_path, {"run_id": RUN, "thread_id": THREAD, "session_ref": REF})
    second = tmp_path / "hosted" / OTHER_AGENT / "turns" / f"{RUN}.json"
    write_turn_record(second, {"run_id": RUN, "thread_id": THREAD, "session_ref": REF})

    def remove(path: Path) -> None:
        if path == first:
            raise OSError(5, "CANARY-0c1d in the message of the error")
        path.unlink()

    monkeypatch.setattr("claude_code_runner.hosting.remove_turn_record", remove)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert first.exists()
    assert not second.exists()
    [line] = _events(logs, "turn_record_unreadable")
    assert (line["record"], line["error"]) == (first.stem, "OSError")
    assert "CANARY" not in logs.text
    # The locks the record's check took are free again.
    for name in (f"thread-{THREAD}", f"session-{REF}"):
        release(await acquire(tmp_path / "hosted" / AGENT / "locks" / f"{name}.lock", 0, 0.01))


async def test_the_identity_is_checked_again_before_the_signal(
    tmp_path: Path, stray: int, logs: LogLines, monkeypatch: pytest.MonkeyPatch
) -> None:
    # The first check finds the turn. Before the signal goes out the process ID belongs to
    # another process: the check runs again right before the signal, and nothing is sent.
    _leftover(tmp_path, stray)
    read = adoption.read_identity
    seen: list[int] = []

    def changed(pid: int, timeout: float) -> ProcessIdentity | None:
        seen.append(pid)
        found = read(pid, timeout)
        if found is None or len(seen) == 1:
            return found
        return ProcessIdentity("Thu Jan  1 00:00:00 1970", found.command)

    monkeypatch.setattr(adoption, "read_identity", changed)
    await stop_leftover_turns(tmp_path, **_GRACE)
    assert len(seen) >= 2
    assert _alive(stray)
    assert _events(logs, "stop_escalated") == []
