# SPDX-License-Identifier: Apache-2.0
"""Adoption decisions and the identity check before any signal (design note 12)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.adoption import (
    MAX_PROCESS_ID,
    AdoptedGroup,
    AdoptionAction,
    adoption_decision,
    names_session,
    process_id,
    process_matches,
    read_identity,
    read_turn_record,
)
from claude_code_runner.handover import handover_records

REF = "0192d3a0-0000-4000-8000-0000000000dd"
RUN = "0192f4a0-7b1c-7d07-84b0-000000000001"
THREAD = "0192f4a0-7b1c-7d07-84b0-000000000002"
CHANNEL = "0192f4a0-7b1c-7d07-84b0-00000000000c"


def test_adoption_decision() -> None:
    assert adoption_decision(alive=True, run_state="running", result_in_log=False) is (
        AdoptionAction.ADOPT
    )
    for ended in ("failed", "cancelled", "completed"):
        assert adoption_decision(alive=True, run_state=ended, result_in_log=True) is (
            AdoptionAction.STOP
        )
    assert adoption_decision(alive=False, run_state="running", result_in_log=True) is (
        AdoptionAction.FINISH_FROM_LOG
    )
    assert adoption_decision(alive=False, run_state="running", result_in_log=False) is (
        AdoptionAction.LEAVE
    )


@pytest.fixture
def child() -> Iterator[subprocess.Popen[bytes]]:
    # A process of this test in its own group, with the session reference in its command line.
    process = subprocess.Popen(
        [sys.executable, "-c", "import sys; sys.stdin.read()", REF],
        stdin=subprocess.PIPE,
        start_new_session=True,
    )
    try:
        yield process
    finally:
        process.kill()
        process.wait()
        if process.stdin is not None:
            process.stdin.close()


def record_of(process: subprocess.Popen[bytes]) -> dict[str, Any]:
    identity = read_identity(process.pid, 30.0)
    assert identity is not None
    return {
        "pid": process.pid,
        "pgid": os.getpgid(process.pid),
        "session_ref": REF,
        "identity": {"lstart": identity.lstart, "command": identity.command},
    }


def test_identity_matches_the_started_process(child: subprocess.Popen[bytes]) -> None:
    assert process_matches(record_of(child), 30.0) is True


def test_identity_mismatch_counts_as_gone(child: subprocess.Popen[bytes]) -> None:
    record = record_of(child)
    assert process_matches({**record, "pgid": record["pgid"] + 1}, 30.0) is False
    other_start = {**record["identity"], "lstart": "Thu Jan  1 00:00:00 1970"}
    assert process_matches({**record, "identity": other_start}, 30.0) is False
    assert process_matches({**record, "session_ref": "another-reference"}, 30.0) is False
    assert process_matches({**record, "pid": 1}, 30.0) is False
    child.kill()
    child.wait()
    assert process_matches(record, 30.0) is False


# ---- a record is a file: nothing in it is taken as given (FIX-RUNNER-HOSTING-REVIEW) ----------


def test_a_process_id_is_an_integer_above_one_that_the_system_can_hold() -> None:
    assert process_id(2) == 2
    assert process_id(MAX_PROCESS_ID) == MAX_PROCESS_ID
    for value in (0, 1, -7, MAX_PROCESS_ID + 1, 2**64, True, 7.0, float("inf"), "7", None, [7]):
        assert process_id(value) is None, value


@pytest.mark.parametrize(
    "changes",
    [
        {"pid": 2**31},  # os.getpgid raises OverflowError for it
        {"pid": 2**64},
        {"pgid": 2**64},
        {"pid": float("inf")},
        {"pid": "4711"},
        {"pid": True, "pgid": True},
        {"pgid": 0},
        {"pgid": 1},
    ],
)
def test_a_process_id_outside_the_range_counts_as_gone(
    child: subprocess.Popen[bytes], changes: dict[str, Any]
) -> None:
    # Neither an error of the caller nor a process the runner would signal.
    record = {**record_of(child), **changes}
    assert process_matches(record, 30.0) is False
    group = AdoptedGroup(record, 30.0)
    assert group.alive() is False
    assert group.may_signal() is False


def test_the_session_is_the_whole_argument_of_its_flag() -> None:
    assert names_session(f"claude -p --session-id {REF} --model m", REF)
    assert names_session(f"/opt/claude code/bin/claude -p --model m --resume {REF}", REF)
    assert not names_session(f"claude -p --model m {REF}", REF)
    assert not names_session(f"claude -p --session-id {REF}-copy", REF)
    assert not names_session(f"claude -p --session-id x{REF}", REF)
    assert not names_session(f"claude -p --resume-from {REF}", REF)
    assert not names_session(f"tail -f sessions/{REF}/run.stream.jsonl", REF)
    assert not names_session("", REF)


def _record_file(tmp_path: Path, record: object, name: str = RUN) -> Path:
    path = tmp_path / f"{name}.json"
    path.write_text(record if isinstance(record, str) else json.dumps(record))
    return path


def test_a_turn_record_is_read_with_its_ids_checked(tmp_path: Path) -> None:
    record = {"run_id": RUN, "thread_id": THREAD, "session_ref": REF, "pid": 0, "more": [1]}
    assert read_turn_record(_record_file(tmp_path, record)) == record
    # A reference in upper case is what the runner takes from the server and passes on.
    upper = {**record, "session_ref": REF.upper()}
    assert read_turn_record(_record_file(tmp_path, upper)) == upper


@pytest.mark.parametrize("phase", ["preparing", "waiting_lock"])
def test_the_record_of_a_run_in_prepare_carries_no_session(tmp_path: Path, phase: str) -> None:
    # Exactly the members `RunnerHandler._phase` writes with `runner.turn_handover` before
    # `run.start`: the session is stored only from the phase `transcribing` on. A next runner
    # removes this record (09 18.19.6), so it has to be one the reader takes.
    record = {"run_id": RUN, "thread_id": THREAD, "channel_id": CHANNEL, "phase": phase}
    assert read_turn_record(_record_file(tmp_path, record)) == record


@pytest.mark.parametrize(
    "changes",
    [
        {},  # written without the key
        {"phase": "transcribing"},
        {"phase": "launching"},
        {"phase": "process"},
        {"phase": "compaction_retry"},
        {"phase": "PREPARING"},
        {"phase": None},
        {"phase": ["preparing"]},
        {"phase": {"preparing": 1}},
        {"phase": "preparing", "session_ref": None},
        {"phase": "preparing", "session_ref": ""},
        {"phase": "waiting_lock", "session_ref": "x/../../escape/evil"},
    ],
)
def test_only_the_record_of_a_run_in_prepare_may_carry_no_session(
    tmp_path: Path, changes: dict[str, Any]
) -> None:
    # Every later phase has a session, whose name a lock carries; and a session reference
    # that is there is checked, whatever the phase.
    record = {"run_id": RUN, "thread_id": THREAD, "channel_id": CHANNEL, **changes}
    assert read_turn_record(_record_file(tmp_path, record)) is None


@pytest.mark.parametrize(
    "changes",
    [
        {"run_id": "0192f4a0-7b1c-7d07-84b0-0000000000ff"},  # not the run of its file name
        {"run_id": RUN.upper()},
        {"run_id": 5},
        {"run_id": None},
        {"thread_id": "../../escape"},
        {"thread_id": THREAD.upper()},
        {"thread_id": 5},
        {"session_ref": "x/../../escape/evil"},
        {"session_ref": REF.replace("-", "")},
        {"session_ref": f"{{{REF}}}"},
        {"session_ref": f"urn:uuid:{REF}"},
        {"session_ref": ""},
        {"session_ref": None},
        {"session_ref": 5},
    ],
)
def test_a_turn_record_with_another_id_is_unreadable(
    tmp_path: Path, changes: dict[str, Any]
) -> None:
    record = {"run_id": RUN, "thread_id": THREAD, "session_ref": REF, **changes}
    assert read_turn_record(_record_file(tmp_path, record)) is None


@pytest.mark.parametrize(
    "content",
    ["{", "[]", "7", '"text"', "null", "[" * 100_000, "\xff"],
    ids=["cut_off", "list", "number", "text", "null", "nested_too_deep", "no_utf8"],
)
def test_a_file_that_is_no_json_object_is_no_turn_record(tmp_path: Path, content: str) -> None:
    path = tmp_path / f"{RUN}.json"
    path.write_bytes(content.encode("latin-1"))
    assert read_turn_record(path) is None


def test_only_a_regular_file_is_a_turn_record(tmp_path: Path) -> None:
    record = {"run_id": RUN, "thread_id": THREAD, "session_ref": REF}
    real = _record_file(tmp_path, record, "elsewhere")
    path = tmp_path / f"{RUN}.json"
    assert read_turn_record(path) is None  # missing
    path.symlink_to(real)
    assert read_turn_record(path) is None  # a link is not followed
    path.unlink()
    os.mkfifo(path)
    assert read_turn_record(path) is None  # a named pipe does not hold up the read
    path.unlink()
    path.mkdir()
    assert read_turn_record(path) is None


def test_the_records_of_a_handover_are_listed_by_the_same_reader(tmp_path: Path) -> None:
    # The start asks for the records with a phase before it reads any of them
    # (`handover_records`): the same files, so the same reader.
    turns = tmp_path / "turns"
    turns.mkdir()
    record = {"run_id": RUN, "thread_id": THREAD, "session_ref": REF, "phase": "process"}
    second = "0192f4a0-7b1c-7d07-84b0-000000000005"
    # A run in prepare has a record without a session (`RunnerHandler._phase`); it is a record
    # of a handover like the others, so a start without the key refuses while it is left.
    prepare = "0192f4a0-7b1c-7d07-84b0-000000000007"
    in_prepare = {"run_id": prepare, "thread_id": THREAD, "channel_id": CHANNEL}
    listed = [
        _record_file(turns, record),
        _record_file(turns, {**record, "run_id": second}, second),
        _record_file(turns, {**in_prepare, "phase": "waiting_lock"}, prepare),
    ]
    _record_file(turns, record, "0192f4a0-7b1c-7d07-84b0-000000000003")  # the run of another file
    without = {"run_id": RUN, "thread_id": THREAD, "session_ref": REF}
    fourth = "0192f4a0-7b1c-7d07-84b0-000000000004"
    _record_file(turns, {**without, "run_id": fourth}, fourth)  # written without the key
    os.mkfifo(turns / "0192f4a0-7b1c-7d07-84b0-000000000006.json")  # would hold up the start
    assert handover_records(tmp_path) == listed
