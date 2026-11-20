# SPDX-License-Identifier: Apache-2.0
"""Stage files of a coding chain become steps and the status line of the run (09 17.12 point 9).

The sample ``tests/fixtures/stages/chain_run.jsonl`` was written by the coding chain's own
runner (``runTicket`` of its ``src/runner/runner.ts``, with the store wrapped by
``withStageEvents`` of ``src/chain-command/stage-events.ts``) for the invented bug ticket of the
chain's test material, run in a scratch copy; only the ticket key was set to ``TOY-7``. The run
goes through the stages with an ``ended`` line each, three further passes of the red team, and
ends ``code-done``.
"""

from __future__ import annotations

import asyncio
import json
import logging
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from uuid import UUID, uuid4

import pytest
from adapter_kit import FakeRuntime, Rig, rig

from bothy_hermes_adapter import stages
from bothy_hermes_adapter.__main__ import main as adapter_main
from bothy_hermes_adapter.jobs import JobError
from bothy_hermes_adapter.runtime import TurnOutcome, TurnRequest
from bothy_hermes_adapter.stages import (
    LINE_MAX_BYTES,
    STAGES,
    StageCursor,
    StageEvent,
    StageRegistry,
    StageSteps,
    parse_line,
    read_new,
)

pytestmark = pytest.mark.anyio

FIXTURE = Path(__file__).parent.parent / "fixtures" / "stages" / "chain_run.jsonl"
SAMPLE = FIXTURE.read_bytes().splitlines(keepends=True)
FAST: dict[str, Any] = {
    "background_poll_interval": 0.02,
    "background_settle": 0.3,
    "background_progress_interval": 0.2,
}
CANARY = "SECRET-CONTENT-7f3a"


def line(stage: str, state: str, outcome: str | None = None, *, ticket: str = "TOY-7") -> bytes:
    """A line in the chain's member order (at, ticket, run, stage, state, outcome, text)."""
    value: dict[str, Any] = {
        "at": "2026-11-19T12:00:00.000Z",
        "ticket": ticket,
        "run": "r1763553600000",
        "stage": stage,
        "state": state,
    }
    if outcome is not None:
        value["outcome"] = outcome
    value["text"] = f"{stage} ended: {outcome}" if outcome else f"{stage} {state}"
    return (json.dumps(value, separators=(",", ":")) + "\n").encode()


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
def registry(tmp_path: Path) -> StageRegistry:
    home = tmp_path / "profile"
    (home / "bothy").mkdir(mode=0o700, parents=True)
    reg = StageRegistry(stages.stages_dir(home, "bothy"))
    reg.prepare()
    return reg


class Patcher:
    """Records what `StageSteps` patches."""

    def __init__(self) -> None:
        self.steps: dict[str, dict[str, Any]] = {}
        self.status: list[str] = []

    def add_step(self, kind: str, name: str, **kw: Any) -> UUID:
        step = uuid4()
        self.steps[str(step)] = {"kind": kind, "name": name, **kw}
        return step

    def update_step(self, step: UUID, **kw: Any) -> None:
        self.steps[str(step)].update({k: v for k, v in kw.items() if v is not None})

    def set_status_line(self, text: str) -> None:
        self.status.append(text)


def _now() -> datetime:
    return datetime(2026, 11, 20, tzinfo=UTC)


# ---- lines ---------------------------------------------------------------------------------------


def test_every_sample_line_of_the_chain_is_read() -> None:
    events = [parse_line(raw.rstrip(b"\n")) for raw in SAMPLE]
    assert all(isinstance(e, StageEvent) for e in events)
    texts = [json.loads(raw)["text"] for raw in SAMPLE]
    assert [e.text for e in events if isinstance(e, StageEvent)] == texts


@pytest.mark.parametrize("stage", STAGES)
def test_every_stage_and_state_is_mapped(stage: str) -> None:
    assert parse_line(line(stage, "started").strip()) == StageEvent("TOY-7", stage, "started")
    raw = json.loads(line(stage, "pass"))
    raw["text"] = f"{stage} pass 3"
    assert parse_line(json.dumps(raw).encode()) == StageEvent("TOY-7", stage, "pass", pass_no=3)
    for outcome in stages.STAGE_OUTCOMES:
        event = parse_line(line(stage, "ended", outcome).strip())
        assert event == StageEvent("TOY-7", stage, "ended", outcome)


@pytest.mark.parametrize("end", stages.END_STATES)
def test_every_end_state_of_the_run_is_mapped(end: str) -> None:
    assert parse_line(line("run", "ended", end).strip()) == StageEvent("TOY-7", "run", "ended", end)


@pytest.mark.parametrize(
    ("data", "reason"),
    [
        (b"not json", "malformed"),
        (b"[1, 2]", "malformed"),
        (line("deploy", "started").strip(), "unknown_stage"),
        (line("build", "paused").strip(), "unknown_state"),
        (line("run", "pass").strip(), "unknown_state"),
        (line("build", "started", ticket="no key").strip(), "bad_ticket"),
        (b"x" * (LINE_MAX_BYTES + 1), "too_long"),
    ],
)
def test_a_line_off_the_format_is_skipped_with_a_reason(data: bytes, reason: str) -> None:
    assert parse_line(data) == reason


def test_an_outcome_off_the_lists_is_unknown_and_text_is_never_copied() -> None:
    raw = json.loads(line("plan", "ended", "passed"))
    raw["outcome"], raw["text"] = CANARY, CANARY
    event = parse_line(json.dumps(raw).encode())
    assert event == StageEvent("TOY-7", "plan", "ended", "unknown")
    assert isinstance(event, StageEvent)
    assert CANARY not in event.text
    assert parse_line(line("build", "started").strip(), "TOY-8") == "other_ticket"


# ---- reading -------------------------------------------------------------------------------------


def test_a_partial_line_waits_for_its_end(tmp_path: Path) -> None:
    path = tmp_path / "stages.jsonl"
    cursor = StageCursor(0)
    assert read_new(str(path), cursor) == ([], [])  # no file yet
    path.write_bytes(SAMPLE[0] + SAMPLE[1][:20])
    lines, _ = read_new(str(path), cursor)
    assert lines == [SAMPLE[0].rstrip(b"\n")]
    assert cursor.offset == len(SAMPLE[0])
    assert read_new(str(path), cursor) == ([], [])
    with path.open("ab") as f:
        f.write(SAMPLE[1][20:])
    lines, _ = read_new(str(path), cursor)
    assert lines == [SAMPLE[1].rstrip(b"\n")]


def test_an_endless_line_is_skipped_to_its_end(tmp_path: Path) -> None:
    path = tmp_path / "stages.jsonl"
    path.write_bytes(b"y" * (LINE_MAX_BYTES + 10))
    cursor = StageCursor(0)
    assert read_new(str(path), cursor) == ([], ["too_long"])
    assert cursor.skipping
    with path.open("ab") as f:
        f.write(b"yyy\n" + SAMPLE[0])
    lines, _ = read_new(str(path), cursor)
    assert lines == [SAMPLE[0].rstrip(b"\n")]
    assert not cursor.skipping


def test_a_replaced_or_truncated_file_is_read_from_its_start(tmp_path: Path) -> None:
    path = tmp_path / "stages.jsonl"
    path.write_bytes(b"".join(SAMPLE[:3]))
    cursor = StageCursor(0)
    assert len(read_new(str(path), cursor)[0]) == 3
    path.write_bytes(SAMPLE[5])  # same inode, shorter
    lines, notes = read_new(str(path), cursor)
    assert (lines, notes) == ([SAMPLE[5].rstrip(b"\n")], ["truncated"])
    other = tmp_path / "new.jsonl"
    other.write_bytes(SAMPLE[6] + SAMPLE[7])
    other.replace(path)
    lines, notes = read_new(str(path), cursor)
    assert notes == ["replaced"]
    assert len(lines) == 2


def test_a_link_is_not_followed(tmp_path: Path) -> None:
    target = tmp_path / "elsewhere.jsonl"
    target.write_bytes(SAMPLE[0])
    link = tmp_path / "stages.jsonl"
    link.symlink_to(target)
    with pytest.raises(OSError, match=r"link|Too many levels"):
        read_new(str(link), StageCursor(0))


# ---- the registry --------------------------------------------------------------------------------


def test_registration_starts_after_the_lines_already_there(
    tmp_path: Path, registry: StageRegistry
) -> None:
    path = tmp_path / "stages.jsonl"
    path.write_bytes(b"".join(SAMPLE[:4]))  # an earlier call of the command
    thread = uuid4()
    record = registry.register(thread, path, "TOY-7")
    assert record.offset == path.stat().st_size
    assert registry.collect(thread, "r") == [(record, registry.cursor(record), [])]
    with path.open("ab") as f:
        f.write(SAMPLE[4])
    [(_, cursor, events)] = registry.collect(thread, "r")
    assert [e.text for e in events] == ["triage ended: passed"]
    assert registry.collect(uuid4(), "r") == []  # another thread reads nothing
    registry.save_cursor(record.source_id, cursor)
    # A new process (a restart) goes on from the saved cursor.
    again = StageRegistry(registry.directory)
    [(_, _, events)] = again.collect(thread, "r")
    assert events == []
    modes = {p.name: p.stat().st_mode & 0o777 for p in registry.directory.iterdir()}
    assert set(modes.values()) == {0o600}
    assert registry.directory.stat().st_mode & 0o777 == 0o700


def test_a_new_registration_of_the_file_replaces_the_old_one(
    tmp_path: Path, registry: StageRegistry
) -> None:
    thread = uuid4()
    first = registry.register(thread, tmp_path / "stages.jsonl")
    second = registry.register(thread, tmp_path / "stages.jsonl")
    assert [r.source_id for r in registry.records()] == [second.source_id]
    assert registry.retire(first.source_id) is False


def test_a_stale_registration_is_removed(tmp_path: Path) -> None:
    home = tmp_path / "p" / "bothy"
    home.mkdir(mode=0o700, parents=True)
    clock = [1000.0]
    reg = StageRegistry(home / "stages", clock=lambda: clock[0])
    old = reg.register(uuid4(), tmp_path / "a.jsonl")
    clock[0] += stages.SOURCE_MAX_AGE_S + 1
    new = reg.register(uuid4(), tmp_path / "b.jsonl")
    assert [r.source_id for r in reg.records()] == [new.source_id]
    assert old.source_id != new.source_id


def test_a_registration_is_refused_without_a_state_dir_or_with_bad_input(tmp_path: Path) -> None:
    reg = StageRegistry(tmp_path / "missing" / "stages")
    with pytest.raises(JobError):
        reg.register(uuid4(), tmp_path / "stages.jsonl")
    (tmp_path / "ok").mkdir(mode=0o700)
    reg = StageRegistry(tmp_path / "ok" / "stages")
    with pytest.raises(JobError):
        reg.register(uuid4(), "relative/stages.jsonl")
    with pytest.raises(JobError):
        reg.register(uuid4(), tmp_path / "stages.jsonl", ticket="no key")


def test_a_foreign_record_is_skipped(registry: StageRegistry, tmp_path: Path) -> None:
    bad = registry.directory / ("a" * 32 + ".json")
    bad.write_text("{}", encoding="utf-8")
    bad.chmod(0o644)
    assert registry.records() == []


def test_the_command_line_registers_lists_and_deregisters(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    home = tmp_path / "profile"
    (home / "bothy").mkdir(mode=0o700, parents=True)
    file = str(tmp_path / "stages.jsonl")
    thread = str(uuid4())
    base = ["--profile-home", str(home)]
    assert adapter_main(["stages", "register", *base, "--thread", thread, "--file", file]) == 0
    registered = json.loads(capsys.readouterr().out)
    assert registered["ok"] is True
    assert adapter_main(["stages", "list", *base]) == 0
    listed = json.loads(capsys.readouterr().out)
    assert (listed["thread_id"], listed["path"]) == (thread, file)
    sid = registered["source_id"]
    assert adapter_main(["stages", "deregister", *base, "--source-id", sid]) == 0
    capsys.readouterr()
    assert adapter_main(["stages", "deregister", *base, "--source-id", sid]) == 1
    capsys.readouterr()
    assert adapter_main(["stages", "register", *base, "--thread", "x", "--file", file]) == 2
    assert json.loads(capsys.readouterr().out) == {
        "ok": False,
        "error": "the thread is no thread ID",
    }


# ---- the mapping ---------------------------------------------------------------------------------


def test_the_sample_run_becomes_one_step_per_stage() -> None:
    patch = Patcher()
    steps = StageSteps(patch, _now, {})
    ended = [
        steps.apply(e) for raw in SAMPLE if isinstance(e := parse_line(raw.strip()), StageEvent)
    ]
    assert ended[-1] is True
    assert not any(ended[:-1])
    shown = [(s["title"], s["status"]) for s in patch.steps.values()]
    assert shown == [
        ("intake: passed", "completed"),
        ("triage: passed", "completed"),
        ("plan: passed-direct", "completed"),
        ("base-check: passed", "completed"),
        ("build: passed", "completed"),
        ("evidence: passed", "completed"),
        ("red-team: passed", "completed"),
        ("ship: passed", "completed"),
    ]
    red_team = [s for s in patch.steps.values() if s["detail"]["stage"] == "red-team"]
    assert red_team[0]["detail"] == {"stage": "red-team", "outcome": "passed"}
    assert {(s["kind"], s["name"]) for s in patch.steps.values()} == {("workflow", "chain_stage")}
    assert all(s.get("finished_at") for s in patch.steps.values())
    assert patch.status[0] == "TOY-7: run started"
    assert patch.status[-1] == "TOY-7: run ended: code-done"
    assert steps.open == {}


def test_a_start_ends_an_open_step_of_another_stage_and_a_pass_renames() -> None:
    patch = Patcher()
    steps = StageSteps(patch, _now, {})
    raw = json.loads(line("build", "pass"))
    raw["text"] = "build pass 2"
    for data in (line("build", "started"), line("evidence", "started"), json.dumps(raw).encode()):
        event = parse_line(data.strip())
        assert isinstance(event, StageEvent)
        steps.apply(event)
    shown = [(s["title"], s["status"]) for s in patch.steps.values()]
    assert shown == [
        ("build", "completed"),
        ("evidence", "completed"),
        ("build (pass 2)", "running"),
    ]


@pytest.mark.parametrize(
    ("outcome", "stage_status", "end", "open_status"),
    [
        ("failed", "failed", "failed", "failed"),
        ("red", "failed", "halted", "cancelled"),
        ("needs-human", "completed", "needs-human", "completed"),
        ("unknown", "completed", "base-red", "failed"),
    ],
)
def test_outcomes_and_ends_set_the_step_status(
    outcome: str, stage_status: str, end: str, open_status: str
) -> None:
    patch = Patcher()
    steps = StageSteps(patch, _now, {})
    for raw in (line("plan", "started"), line("plan", "ended", outcome), line("build", "started")):
        event = parse_line(raw.strip())
        assert isinstance(event, StageEvent)
        steps.apply(event)
    event = parse_line(line("run", "ended", end).strip())
    assert isinstance(event, StageEvent)
    assert steps.apply(event) is True
    plan, build = patch.steps.values()
    assert (plan["title"], plan["status"]) == (f"plan: {outcome}", stage_status)
    assert plan["detail"] == {"stage": "plan", "outcome": outcome}
    assert (build["title"], build["status"]) == ("build", open_status)


# ---- in the run ----------------------------------------------------------------------------------


def _steps(r: Rig, run_id: UUID) -> list[dict[str, Any]]:
    run = r.fake.run(run_id)
    found = [s for s in run["steps"].values() if s["name"] == stages.STEP_NAME]
    return sorted(found, key=lambda s: s["ordinal"])


def _chain(registry: StageRegistry, path: Path, writes: list[bytes], hold: asyncio.Event) -> Any:
    """A turn whose tool call registers the stage file and runs a chain that writes ``writes``."""

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            registry.register(turn.thread_id, path, "TOY-7")
            for chunk in writes:
                with path.open("ab") as f:
                    f.write(chunk)
                await asyncio.sleep(0.05)
            await hold.wait()
        await rt.events.on_answer(turn.thread_id, "done", str(turn.message_id))
        return TurnOutcome.COMPLETED

    return script


async def test_the_run_shows_the_chain_while_the_turn_runs(
    tmp_path: Path, registry: StageRegistry, caplog: pytest.LogCaptureFixture
) -> None:
    path = tmp_path / "run" / "stages.jsonl"
    path.parent.mkdir()
    junk = [
        b"not json " + CANARY.encode() + b"\n",
        line("deploy", "started"),
        line("build", "started", ticket="TOY-8"),
    ]
    # A line split over two writes, junk in the middle, then the rest of the sample.
    writes = [SAMPLE[0] + SAMPLE[1][:30], SAMPLE[1][30:], *junk, *SAMPLE[2:]]
    hold = asyncio.Event()
    rt = FakeRuntime(script=_chain(registry, path, writes, hold))
    caplog.set_level(logging.INFO, logger="bothy_hermes_adapter")
    async with rig(tmp_path, runtime=rt, stages=registry, **FAST) as r:
        triggered = await r.fake.trigger()

        def ended() -> bool:
            return r.fake.run(triggered.run_id).get("status_line") == "TOY-7: run ended: code-done"

        await r.until(ended, "the chain's end in the status line")
        # The steps show while the turn still runs.
        assert r.fake.run_state(triggered.run_id).state == "running"
        assert len(_steps(r, triggered.run_id)) == 8
        hold.set()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert {s["status"] for s in _steps(r, triggered.run_id)} == {"completed"}
    assert registry.records() == []  # the chain's end retired the source
    for code in ("malformed", "unknown_stage", "other_ticket"):
        assert f": {code}" in caplog.text
    assert CANARY not in caplog.text
    assert str(path) not in caplog.text


async def test_the_end_of_the_run_ends_the_open_steps_and_the_reading(
    tmp_path: Path, registry: StageRegistry
) -> None:
    path = tmp_path / "stages.jsonl"
    hold = asyncio.Event()
    hold.set()
    rt = FakeRuntime(script=_chain(registry, path, [b"".join(SAMPLE[:8])], hold))
    async with rig(tmp_path, runtime=rt, stages=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        shown = [(s["title"], s["status"]) for s in _steps(r, triggered.run_id)]
        assert shown[-1] == ("base-check", "cancelled")
        assert all(status != "running" for _, status in shown)
        patches = len(r.requests("patchRun"))
        with path.open("ab") as f:
            f.write(b"".join(SAMPLE[8:]))
        await asyncio.sleep(0.2)
        assert len(r.requests("patchRun")) == patches  # nothing after the run's end
    [source] = registry.records()
    cursor = registry.cursor(source)
    assert cursor.offset == len(b"".join(SAMPLE[:8]))
    assert cursor.open == {}


async def test_the_next_run_of_the_thread_goes_on_from_the_cursor(
    tmp_path: Path, registry: StageRegistry
) -> None:
    path = tmp_path / "stages.jsonl"
    hold = asyncio.Event()
    hold.set()
    rt = FakeRuntime(script=_chain(registry, path, [b"".join(SAMPLE[:8])], hold))
    async with rig(tmp_path, runtime=rt, stages=registry, **FAST) as r:
        first = await r.fake.trigger()
        await r.until(lambda: r.finished(first.run_id), "the end of the first run")
        # The chain goes on while no run is open (the gateway restarted, for example).
        with path.open("ab") as f:
            f.write(b"".join(SAMPLE[8:]))
        second = await r.fake.trigger("How far is it?", thread_id=first.thread_id)
        await r.until(lambda: r.finished(second.run_id), "the end of the second run")
    titles = [s["title"] for s in _steps(r, second.run_id)]
    assert titles == [
        "base-check: passed",
        "build: passed",
        "evidence: passed",
        "red-team: passed",
        "ship: passed",
    ]
    assert r.fake.run(second.run_id)["status_line"] == "TOY-7: run ended: code-done"
    assert registry.records() == []


async def test_a_run_of_another_thread_reads_nothing(
    tmp_path: Path, registry: StageRegistry
) -> None:
    path = tmp_path / "stages.jsonl"
    path.write_bytes(b"")
    registry.register(uuid4(), path)
    path.write_bytes(b"".join(SAMPLE))
    async with rig(tmp_path, stages=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert _steps(r, triggered.run_id) == []
    assert len(registry.records()) == 1


async def test_a_new_registration_in_the_run_ends_the_old_sources_steps(
    tmp_path: Path, registry: StageRegistry
) -> None:
    path = tmp_path / "stages.jsonl"

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        # The first call of the chain stops after its intake started (no end line).
        registry.register(turn.thread_id, path, "TOY-7")
        with path.open("ab") as f:
            f.write(b"".join(SAMPLE[:2]))
        await asyncio.sleep(0.3)
        # The agent calls the chain again for the same ticket in the same run.
        registry.register(turn.thread_id, path, "TOY-7")
        with path.open("ab") as f:
            f.write(SAMPLE[2] + SAMPLE[-1])
        await asyncio.sleep(0.3)
        await rt.events.on_answer(turn.thread_id, "done", str(turn.message_id))
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, stages=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    shown = [(s["title"], s["status"]) for s in _steps(r, triggered.run_id)]
    assert shown == [("intake", "cancelled"), ("intake: passed", "completed")]


async def test_a_cursor_that_cannot_be_saved_shows_no_line_twice(
    tmp_path: Path, registry: StageRegistry, caplog: pytest.LogCaptureFixture
) -> None:
    path = tmp_path / "stages.jsonl"
    hold = asyncio.Event()

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        registry.register(turn.thread_id, path, "TOY-7")
        registry.directory.chmod(0o500)  # a full disk, for example: no cursor is written
        with path.open("ab") as f:
            f.write(b"".join(SAMPLE[:4]))
        await hold.wait()
        await rt.events.on_answer(turn.thread_id, "done", str(turn.message_id))
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    caplog.set_level(logging.WARNING, logger="bothy_hermes_adapter")
    try:
        async with rig(tmp_path, runtime=rt, stages=registry, **FAST) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: len(_steps(r, triggered.run_id)) >= 2, "the steps")
            await asyncio.sleep(0.3)  # many polls
            hold.set()
            await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    finally:
        registry.directory.chmod(0o700)
    shown = [(s["title"], s["status"]) for s in _steps(r, triggered.run_id)]
    assert shown == [("intake: passed", "completed"), ("triage", "cancelled")]
    assert "stage cursor not saved" in caplog.text
