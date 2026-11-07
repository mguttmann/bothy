# SPDX-License-Identifier: Apache-2.0
"""`ProcessState` and its replay (docs/spec/09-agents.md 18.19.7, AR-T02, design note 2.5).

Every value the stdin and stop rules read and that stream lines produce lives in one object per
process. The adopter replays the lines from ``process_start`` to the record's offset through the
same ``apply`` with the adoption time, then reads on. The cases of the design note (quiet window,
offset past the result, error result before the checkpoint, task running) and the class test of
the brief (design note 4, issue 556 item 3): for every stream the Fake-Claude writes and every
checkpoint position in it, the old runner's read and the adopter's replay plus read give the same
sequence of stdin and stop decisions, with the adopter's times never earlier than the old
runner's shifted by the adoption.
"""

from __future__ import annotations

import json
import sys
from collections.abc import Callable, Iterator
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.processstate import Decision, ProcessState, replayed_lines

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import fake_claude

FOLLOWUP_GRACE = 30.0
STOP_GRACE = 10.0
STEP = 1.0
"""The poll step of the simulation (seconds of the fake clock)."""

Stream = list[dict[str, Any]]


# ---- the streams Fake-Claude writes -----------------------------------------------------------


class _Recorder(fake_claude.Fake):
    """A Fake-Claude whose messages are collected instead of printed."""

    def __init__(self, scenario: dict[str, Any], capture: Path) -> None:
        super().__init__(scenario, capture)
        self.out: Stream = []
        self.session_id = "0192f4a0-0000-7000-8000-00000000c0de"
        self.model = "model-large"

    def emit(self, message: dict[str, Any]) -> None:
        self.out.append(json.loads(json.dumps(message)))


def _line(uuid: str, text: str) -> str:
    doc = {
        "type": "user",
        "uuid": uuid,
        "message": {"role": "user", "content": [{"type": "text", "text": text}]},
    }
    return json.dumps(doc)


def _turn(fake: _Recorder, fixture: str, uuid: str) -> None:
    """One turn as `Fake.turn` plays it without gates: init, echo, the fixture, its result."""
    fake.scenario = {**fake.scenario, "fixture": fixture}
    fake.emit(fake.init_line())
    fake.echo(_line(uuid, "Fix the form"))
    result = None
    for line in fake.fixture():
        if line.get("type") == "result":
            result = line
            continue
        fake.emit(line)
    final = dict(
        result or {"type": "result", "subtype": "success", "is_error": False, "result": ""}
    )
    final["user_message_uuids"] = [uuid]
    fake.emit(final)


def fake_streams(tmp: Path) -> dict[str, Stream]:
    """Every stream shape of the scenarios, written by Fake-Claude's own code."""
    capture = tmp / "capture.jsonl"
    streams: dict[str, Stream] = {}
    fixtures = sorted(p.stem for p in fake_claude.FIXTURES.glob("*.jsonl"))
    for name in fixtures:
        fake = _Recorder({}, capture)
        _turn(fake, name, "u-prompt")
        streams[name] = fake.out
    # A background job that outlives the turn; a follow-up while it runs; then its end with the
    # notification turn (FIX-runner-responsive-turns, AR-T02).
    fake = _Recorder({}, capture)
    _turn(fake, "plain", "u-prompt")
    fake.out.pop()  # the result comes after the job started
    fake.start_background()
    fake.emit({"type": "result", "subtype": "success", "is_error": False, "result": "Started."})
    fake.emit(fake.init_line())
    fake.echo(_line("u-follow", "Status?"))
    fake.emit({**fake.followup_result(1, None), "user_message_uuids": ["u-follow"]})
    fake.background_done()
    streams["background"] = fake.out
    # An error result while the job runs (AR-T02 (c), FIX-runner-auth-failed-held).
    fake = _Recorder({}, capture)
    _turn(fake, "plain", "u-prompt")
    fake.out.pop()
    fake.start_background()
    fake.emit({"type": "result", "subtype": "success", "is_error": False, "result": "Started."})
    fake.emit(fake.init_line())
    fake.emit_api_error("API Error: 401 authentication failed")
    streams["background_error"] = fake.out
    # A resume after lost background work: results before the queued input started.
    fake = _Recorder({}, capture)
    fake.lost_work_turns("cmd-0", 2)
    _turn(fake, "plain", "u-prompt")
    fake.emit({"type": "command_lifecycle", "command_uuid": "cmd-0", "state": "completed"})
    streams["lost_work"] = fake.out
    # A follow-up queued while a turn runs, started after its result (lifecycle).
    fake = _Recorder({}, capture)
    _turn(fake, "plain", "u-prompt")
    for state in ("queued", "started"):
        fake.emit({"type": "command_lifecycle", "command_uuid": "cmd-1", "state": state})
    fake.echo(_line("u-follow", "Also the footer"))
    fake.emit({**fake.followup_result(1, None), "user_message_uuids": ["u-follow"]})
    streams["lifecycle"] = fake.out
    return streams


# ---- the simulation of a read -----------------------------------------------------------------


@dataclass
class Read:
    """One read of a process stream on a fake clock: the decisions with their times."""

    decisions: list[tuple[float, Decision]]
    stdin_open: bool
    stopped: bool


def _decide(state: ProcessState, now: float, stdin_open: bool) -> Decision:
    return state.decide(
        now,
        stdin_open=stdin_open,
        pending=0,
        followup_grace=FOLLOWUP_GRACE,
        stop_grace=STOP_GRACE,
    )


def simulate(
    state: ProcessState,
    lines: list[tuple[float, dict[str, Any]]],
    *,
    stdin_open: bool,
    start: float,
    until: float,
) -> Read:
    """Read ``lines`` (time, message) like `RunTurn._read`: a decision at every result line
    and at every poll between lines, from ``start`` until ``until``."""
    out: list[tuple[float, Decision]] = []
    stopped = False

    def act(now: float) -> bool:
        nonlocal stdin_open, stopped
        decision = _decide(state, now, stdin_open)
        if decision in (Decision.HOLD, Decision.CLOSE, Decision.STOP):
            out.append((now, decision))
        if decision is Decision.CLOSE:
            stdin_open = False
        if decision is Decision.STOP:
            stopped = True
        return stopped

    now = start
    act(now)
    for at, message in lines:
        while now + STEP < at and not stopped:
            now += STEP
            act(now)
        if stopped:
            break
        now = at
        state.apply(message, now)
        if message.get("type") == "result" and act(now):
            break
    while not stopped and now + STEP <= until:
        now += STEP
        act(now)
    return Read(out, stdin_open, stopped)


def _kinds(decisions: list[tuple[float, Decision]]) -> list[tuple[float, Decision]]:
    """Consecutive equal decisions collapsed to the first."""
    out: list[tuple[float, Decision]] = []
    for at, kind in decisions:
        if not out or out[-1][1] is not kind:
            out.append((at, kind))
    return out


def compare(stream: Stream, k: int, gap: float) -> None:
    """The old runner's read against an adoption after line ``k - 1`` (``gap`` seconds later)."""
    times = [100.0 + 5.0 * i for i in range(len(stream))]
    end = times[-1] + 3 * (FOLLOWUP_GRACE + STOP_GRACE)
    old_state = ProcessState(has_stdin=True)
    old = simulate(
        old_state,
        list(zip(times, stream, strict=True)),
        stdin_open=True,
        start=times[0] - STEP,
        until=end,
    )
    handover = times[k - 1] if k else times[0] - STEP
    before = [(t, d) for t, d in old.decisions if t <= handover]
    if any(d is Decision.STOP for _, d in before):
        return  # the old runner stopped the group before the handover: nothing to adopt
    stdin_open = not any(d is Decision.CLOSE for _, d in before)
    after = [(t + gap, d) for t, d in old.decisions if t > handover]
    adoption = handover + gap
    adopter = ProcessState(has_stdin=True)
    adopter.replay(stream[:k], adoption)
    new = simulate(
        adopter,
        [(t + gap, m) for t, m in zip(times[k:], stream[k:], strict=True)],
        stdin_open=stdin_open,
        start=adoption,
        until=end + gap,
    )
    old_kinds, new_kinds = _kinds(after), _kinds(new.decisions)
    # The windows restart at the adoption: the adopter may hold where the old runner's window
    # had just run out (held longer, never shorter); what follows must be the same.
    held = new_kinds[:1] and new_kinds[0][1] is Decision.HOLD
    if held and (not old_kinds or old_kinds[0][1] is not Decision.HOLD):
        new_kinds = new_kinds[1:]
    assert [d for _, d in new_kinds] == [d for _, d in old_kinds], (k, new_kinds, old_kinds)
    for (new_at, kind), (old_at, _) in zip(new_kinds, old_kinds, strict=True):
        if kind is not Decision.HOLD:  # a close or stop never comes earlier than before
            assert new_at >= old_at, (k, new_kinds, old_kinds)
    assert new.stopped == old.stopped


@pytest.fixture(scope="module")
def streams(tmp_path_factory: pytest.TempPathFactory) -> dict[str, Stream]:
    return fake_streams(tmp_path_factory.mktemp("streams"))


def _positions(streams: dict[str, Stream]) -> Iterator[tuple[str, int]]:
    for name, stream in streams.items():
        for k in range(len(stream) + 1):
            yield name, k


def test_the_class_test_covers_every_stream_and_checkpoint(streams: dict[str, Stream]) -> None:
    # Design note 4 of the brief: every stream of the scenarios and every checkpoint position;
    # a value kept outside ProcessState changes the sequence and fails here.
    checked = 0
    for name, k in _positions(streams):
        for gap in (0.0, 7.0, 3600.0):
            compare(streams[name], k, gap)
            checked += 1
    assert checked == 3 * sum(len(s) + 1 for s in streams.values())
    assert {"background", "background_error", "lost_work", "lifecycle", "happy"} <= set(streams)


def test_a_value_kept_outside_fails_the_class_test(
    streams: dict[str, Stream], monkeypatch: pytest.MonkeyPatch
) -> None:
    # The test itself: a replay that forgets the background work (as the adoption before
    # AR-03 did) changes the decisions for the background stream.
    real = ProcessState.replay

    def forgetful(self: ProcessState, lines: Stream, now: float) -> int:
        count = real(self, lines, now)
        self.background.running = []
        self.background.seen = False
        return count

    monkeypatch.setattr(ProcessState, "replay", forgetful)
    stream = streams["background"]
    k = next(i for i, m in enumerate(stream) if m.get("result") == "Started.") + 1
    with pytest.raises(AssertionError):
        compare(stream, k, 7.0)


# ---- the cases of the design note 2.5 ---------------------------------------------------------


def _times(stream: Stream) -> list[tuple[float, dict[str, Any]]]:
    return [(100.0 + i, m) for i, m in enumerate(stream)]


def test_quiet_window_restarts_at_the_adoption(streams: dict[str, Stream]) -> None:
    # Case 1: the job ended, its notification turn was answered, restart inside the quiet
    # window, no further input: stdin closes runner.followup_grace after the adoption.
    stream = streams["background"]
    adopter = ProcessState(has_stdin=True)
    assert adopter.replay(stream, 200.0) == len(stream)
    assert _decide(adopter, 200.0, True) is Decision.HOLD
    assert _decide(adopter, 200.0 + FOLLOWUP_GRACE - 1, True) is Decision.HOLD
    assert _decide(adopter, 200.0 + FOLLOWUP_GRACE, True) is Decision.CLOSE
    assert adopter.last_result_at == 200.0


def test_offset_past_the_result_is_replayed_the_same(streams: dict[str, Stream]) -> None:
    # Case 2: the offset lies past the last result: the replay alone restores the window.
    stream = streams["background"]
    adopter = ProcessState(has_stdin=True)
    adopter.replay(stream, 200.0)
    without = ProcessState(has_stdin=True)
    assert _decide(without, 200.0, True) is Decision.NONE  # today's adoption never closes
    assert _decide(adopter, 200.0 + FOLLOWUP_GRACE, True) is Decision.CLOSE


def test_error_result_before_the_checkpoint_stops_after_the_grace(
    streams: dict[str, Stream],
) -> None:
    # Case 3: an error result while a task runs, before the checkpoint: the adopter stops the
    # group runner.stop_grace after the adoption, with the lost task counted.
    stream = streams["background_error"]
    adopter = ProcessState(has_stdin=True)
    adopter.replay(stream, 200.0)
    assert adopter.failed_at == 200.0
    assert adopter.lost_tasks == 1
    assert _decide(adopter, 205.0, False) is Decision.NONE
    assert _decide(adopter, 200.0 + STOP_GRACE, False) is Decision.STOP
    assert adopter.lost_at_stop() == 1


def test_task_running_keeps_stdin_open(streams: dict[str, Stream]) -> None:
    # Case 4: a task still runs: stdin stays open, also long after the adoption.
    stream = streams["background"]
    k = next(i for i, m in enumerate(stream) if m.get("result") == "Started.") + 1
    adopter = ProcessState(has_stdin=True)
    adopter.replay(stream[:k], 200.0)
    assert adopter.background.running == [fake_claude.BACKGROUND_TASK]
    assert adopter.tasks == {fake_claude.BACKGROUND_TASK: "Run the test suite"}
    assert _decide(adopter, 3600.0, True) is Decision.HOLD


def test_a_turn_without_stdin_gets_no_failure_mark(streams: dict[str, Stream]) -> None:
    # A record without a named pipe is adopted as 18.9 says: reading rights, no stop rule.
    state = ProcessState(has_stdin=False)
    state.replay(streams["background_error"], 200.0)
    assert state.failed_at is None
    assert _decide(state, 10_000.0, False) is Decision.NONE


def test_a_queued_input_waits_for_its_turn(streams: dict[str, Stream]) -> None:
    stream = streams["lost_work"]
    k = next(i for i, m in enumerate(stream) if m.get("type") == "result") + 1
    state = ProcessState(has_stdin=True)
    for at, message in _times(stream[:k]):
        state.apply(message, at)
    assert state.input_waiting
    assert _decide(state, 10_000.0, True) is Decision.WAIT


# ---- the outage number (issue 556 comment, design note 6 of the brief) ------------------------


def _frame(status: str, overage: str = "rejected", resets: int = 1_700_000_000) -> dict[str, Any]:
    return {
        "type": "rate_limit_event",
        "rate_limit_info": {
            "status": status,
            "overageStatus": overage,
            "resetsAt": resets,
            "rateLimitType": "five_hour",
        },
    }


def test_outage_number_starts_from_the_launching_record() -> None:
    # The attempt's ProcessState starts from the highest number reached before it (18.19.5),
    # in the live read and in the replay alike; an outage counts once until a frame clears it.
    frames = [
        _frame("rejected"),
        _frame("rejected", resets=1_700_000_500),
        {"type": "result", "subtype": "success", "is_error": True, "result": "limit"},
        _frame("allowed"),
        _frame("rejected"),
    ]
    live = ProcessState(has_stdin=True, outage=2)
    for at, message in _times(frames):
        live.apply(message, at)
    replayed = ProcessState(has_stdin=True, outage=2)
    replayed.replay(frames, 500.0)
    for state in (live, replayed):
        assert state.outage == 4
        assert state.usage is not None
        assert state.usage.resets_at == 1_700_000_000
        assert state.usage_error  # an error result under the mark is a usage error
        assert state.failed_at is None  # never the failure mark
    # An adopter of the same attempt starts from the same launching record: its replay up to
    # the checkpoint plus its own read reach the old runner's number, never a further one.
    split = ProcessState(has_stdin=True, outage=2)
    split.replay(frames[:2], 600.0)
    for at, message in _times(frames[2:]):
        split.apply(message, 600.0 + at)
    assert split.outage == live.outage


def test_usage_frame_with_overage_or_unknown_status_sets_no_mark() -> None:
    state = ProcessState(has_stdin=True)
    state.apply(_frame("rejected", overage="allowed"), 1.0)
    state.apply(_frame("weird"), 2.0)
    state.apply(_frame("allowed_warning"), 3.0)
    assert state.usage is None
    assert state.outage == 0


# ---- nudge mark and autonomous inputs ----------------------------------------------------------


def test_nudge_mark_and_autonomous_count_follow_the_echoes() -> None:
    state = ProcessState(has_stdin=True, nudges={"u-nudge"}, autonomous={"u-wake"})
    echo: Callable[[str], dict[str, Any]] = lambda u: {  # noqa: E731
        "type": "user",
        "isReplay": True,
        "uuid": u,
    }
    state.apply(echo("u-wake"), 1.0)
    assert (state.autonomous_inputs, state.nudged, state.idle) == (1, False, True)
    state.apply(
        {
            "type": "assistant",
            "message": {"content": [{"type": "tool_use", "name": "Bash", "input": {}}]},
        },
        2.0,
    )
    assert state.idle is False
    state.apply(echo("u-nudge"), 3.0)
    assert state.nudged
    state.apply(echo("u-person"), 4.0)
    assert (state.nudged, state.autonomous_inputs) == (False, 0)


# ---- reading the lines of a replay from the stream file ----------------------------------------


@pytest.mark.anyio
async def test_replayed_lines_lie_between_process_start_and_offset(tmp_path: Path) -> None:
    lines = [
        {"type": "system", "subtype": "init"},
        {"type": "assistant", "message": {"content": []}},
        {"type": "unknown_kind"},
        {"type": "result", "subtype": "success"},
    ]
    earlier = (
        b'{"type":"system","subtype":"background_tasks_changed","tasks":[{"task_id":"old"}]}\n'
    )
    body = b"".join(json.dumps(m).encode() + b"\n" for m in lines)
    path = tmp_path / "s.jsonl"
    path.write_bytes(earlier + body + b'{"type":"result"')
    start = len(earlier)
    offset = start + len(json.dumps(lines[0])) + 1 + len(json.dumps(lines[1])) + 1
    got = await replayed_lines(path, start, offset)
    assert [m for m, _, _ in got] == lines[:2]  # the earlier process is never replayed
    assert got[0][1] == start
    everything = await replayed_lines(path, start, path.stat().st_size)
    assert [m["type"] for m, _, _ in everything] == ["system", "assistant", "result"]
