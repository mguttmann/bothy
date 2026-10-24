# SPDX-License-Identifier: Apache-2.0
"""One Fake-Claude scenario per row of 09 18.9 "Error paths" (exit criterion 5 of M2).

The rows "Crash of the runner" run in `test_crash.py`, because the runner there is a process of
its own and the FakeServer clock follows the real time.
"""

from __future__ import annotations

import json
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.recovery import RETRY_HINT

from .conftest import World

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run on its own timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE}


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def status_steps(world: World, run_id: UUID) -> list[dict[str, Any]]:
    steps = sorted(world.fake.run(run_id)["steps"].values(), key=lambda s: s["ordinal"])
    return [s for s in steps if s["kind"] == "status"]


def lotse_messages(world: World) -> list[dict[str, Any]]:
    """The bodies of the runner's ``createMessage`` calls (its answers)."""
    return list(world.fake.messages)


def flag(argv: list[str], name: str) -> str | None:
    return argv[argv.index(name) + 1] if name in argv else None


def text_of(line: str) -> str:
    return str(json.loads(line)["message"]["content"][0]["text"])


async def test_context_full_compacts_once_and_retries(world: World) -> None:
    world.scenario(
        fixture="happy",
        per_process={"0": {"api_error": "context_full", "api_error_after": 1}},
    )
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    first, compaction, retry = world.turns()
    ref = flag(first["argv"], "--session-id")
    assert flag(compaction["argv"], "--resume") == ref
    assert flag(retry["argv"], "--resume") == ref
    assert [text_of(x) for x in compaction["stdin"]] == ["/compact"]
    assert retry["stdin"][0] == first["stdin"][0]
    assert RETRY_HINT in retry["system_prompt"]
    assert RETRY_HINT not in first["system_prompt"]
    patches = json.dumps([r.json for r in world.requests("patchRun")])
    assert "COMPACTION-OUTPUT-NEVER-POSTED" not in patches
    assert all("COMPACTION-OUTPUT" not in m["content_md"] for m in world.fake.messages)
    notes = [(s["name"], s["status"]) for s in status_steps(world, run.run_id)]
    assert notes == [("context_compacted", "completed")]
    assert len(world.requests("startRun")) == 1
    assert len(lotse_messages(world)) == 1
    assert [f["state"] for f in finishes(world, run.run_id)] == ["completed"]


async def test_context_full_before_work_retries_without_hint(world: World) -> None:
    world.scenario(fixture="plain", per_process={"0": {"api_error": "context_full"}})
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    first, _, retry = world.turns()
    assert RETRY_HINT not in retry["system_prompt"]
    assert retry["system_prompt"] == first["system_prompt"]


async def test_context_full_twice_fails_without_second_compaction(world: World) -> None:
    error = {"api_error": "context_full", "api_error_after": 1}
    world.scenario(fixture="happy", per_process={"0": error, "2": error})
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert len(world.turns()) == 3
    assert len(world.calls("compact")) == 1
    notes = status_steps(world, run.run_id)
    assert [(s["name"], s["status"]) for s in notes] == [
        ("context_compacted", "completed"),
        ("runtime_error", "failed"),
    ]
    assert notes[-1]["detail"] == {"errors": [], "exit": "1"}


async def test_failed_compaction_fails_the_run(world: World) -> None:
    # The pinned CLI ends a failed compaction with a success result but writes no
    # compact_boundary: the run fails runtime_error and no retry starts (D-M2-35-8).
    world.scenario(
        fixture="happy",
        per_process={
            "0": {"api_error": "context_full", "api_error_after": 1},
            "1": {"compact_fails": True},
        },
    )
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert len(world.turns()) == 2
    notes = [(s["name"], s["status"]) for s in status_steps(world, run.run_id)]
    assert notes == [("context_compacted", "failed"), ("runtime_error", "failed")]


async def _started_session(world: World) -> tuple[UUID, str, int]:
    """A first run in a thread; returns the thread, its session reference and row version."""
    world.scenario(fixture="plain")
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    [row] = world.fake.sessions(first.thread_id)
    assert row["started"] is True
    return first.thread_id, row["external_session_ref"], row["version"]


@pytest.mark.parametrize("mode", ["resume", "bind"])
async def test_session_missing_starts_new_session_fail_open(world: World, mode: str) -> None:
    await world.start()
    thread, old, version = await _started_session(world)
    world.scenario(fixture="resume", sessions=True)  # Claude Code knows no session at all
    run = await world.fake.trigger("Add a test", thread_id=thread, session_mode=mode)
    assert await world.ended(run.run_id) == "completed"
    missing, fresh = world.turns()[1:]
    assert flag(missing["argv"], "--resume") == old
    new = flag(fresh["argv"], "--session-id")
    assert new not in (None, old)
    assert "--resume" not in fresh["argv"]
    stored = world.state / "sessions" / str(old) / f"{run.run_id}.input.jsonl"
    assert fresh["stdin"][0] == stored.read_text().rstrip("\n")  # the same first line
    notes = status_steps(world, run.run_id)
    assert [(s["name"], s["status"], s["detail"]) for s in notes] == [
        ("session_missing", "completed", {"mode": mode})
    ]
    reports = world.requests("reportAgentSession")[1:]
    assert len(reports) == 1
    expected: dict[str, Any] = {
        "external_session_ref": new,
        "model": "model-large",
        "started": True,
    }
    if mode == "bind":
        expected["bind_failed"] = "session_missing"
    assert reports[0].json == expected
    assert reports[0].headers["if-match"] == f'"{version}"'
    current = [r for r in world.fake.sessions(thread) if r["is_current"]]
    assert [r["external_session_ref"] for r in current] == [new]


async def test_new_session_fails_too_ends_runtime_error(world: World) -> None:
    await world.start()
    thread, _, _ = await _started_session(world)
    world.scenario(fixture="resume", sessions=True, per_process={"2": {"fail_before_init": True}})
    run = await world.fake.trigger("Add a test", thread_id=thread)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert len(world.turns()) == 3  # the first run, the missing session, the new session
    notes = [s["name"] for s in status_steps(world, run.run_id)]
    assert notes == ["session_missing", "runtime_error"]


async def test_session_exists_on_first_turn_resumes(world: World) -> None:
    await world.start()
    thread, _, _ = await _started_session(world)
    ref = "0192d3a0-0000-4000-8000-0000000000e1"
    world.fake.world.session_for(thread, ref, started=False)  # current, not started: mode new
    world.scenario(fixture="resume", sessions=True, known_sessions=[ref])
    run = await world.fake.trigger("Add a test", thread_id=thread)
    assert await world.ended(run.run_id) == "completed"
    exists, resumed = world.turns()[1:]
    assert flag(exists["argv"], "--session-id") == ref
    assert flag(resumed["argv"], "--resume") == ref
    reports = world.requests("reportAgentSession")[1:]
    assert [r.json["started"] for r in reports] == [True]
    assert reports[0].headers["if-match"] == '"1"'
    assert len([r for r in world.requests("startRun") if str(run.run_id) in r.path]) == 1
    assert status_steps(world, run.run_id) == []


async def test_runtime_not_authenticated(world: World) -> None:
    world.scenario(fixture="plain", auth_method="none")
    await world.start()
    precheck = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(precheck.run_id) == "failed"
    assert world.fake.run_state(precheck.run_id).state_reason == "runtime_not_authenticated"
    assert world.requests("startRun") == []
    assert world.turns() == []
    world.scenario(fixture="plain", api_error="not_authenticated")
    late = await world.fake.trigger("Again", channel_id=world.forum_a)
    assert await world.ended(late.run_id) == "failed"
    assert world.fake.run_state(late.run_id).state_reason == "runtime_not_authenticated"
    assert len(world.turns()) == 1
    assert status_steps(world, late.run_id) == []


async def test_result_error_subtype_fails_with_texts(world: World) -> None:
    world.scenario(fixture="error_texts")
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    [note] = status_steps(world, run.run_id)
    assert (note["name"], note["status"]) == ("runtime_error", "failed")
    assert note["detail"] == {
        "errors": ["Upstream connection reset", "Tool runner stopped"],
        "exit": "0",
    }
    order = [r.operation_id for r in world.fake.requests if str(run.run_id) in r.path]
    note_patch = max(
        i
        for i, r in enumerate(r for r in world.fake.requests if str(run.run_id) in r.path)
        if r.operation_id == "patchRun" and "runtime_error" in json.dumps(r.json)
    )
    assert note_patch < order.index("finishRun")
    assert len(world.turns()) == 1
    assert lotse_messages(world) == []


async def test_quoted_error_text_is_not_an_error_path(world: World) -> None:
    # The model's own words quote the CLI's texts; only the CLI's API error counts (D-M2-35-18).
    world.scenario(fixture="quoted_error")
    await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert len(world.turns()) == 1
    assert world.calls("compact") == []
    [note] = status_steps(world, run.run_id)
    assert note["detail"]["errors"] == ["Reached the maximum number of turns"]
