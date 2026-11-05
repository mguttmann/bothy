# SPDX-License-Identifier: Apache-2.0
"""The background work of a turn process and when it holds stdin open (09 18.5)."""

from __future__ import annotations

from claude_code_runner.background import BackgroundWork

INIT = {"type": "system", "subtype": "init"}
RESULT = {"type": "result", "subtype": "success"}


def tasks(*ids: str, **extra: object) -> dict[str, object]:
    return {
        "type": "system",
        "subtype": "background_tasks_changed",
        "tasks": [{"task_id": i, "task_type": "local_bash", **extra} for i in ids],
    }


def test_a_process_without_background_work_never_holds() -> None:
    work = BackgroundWork()
    for message in (INIT, RESULT):
        work.apply(message, 1.0)
    assert not work.holds(1.0, 30.0)
    assert work.running == []


def test_running_tasks_hold_stdin_open_at_any_time() -> None:
    work = BackgroundWork()
    work.apply(INIT, 0.0)
    work.apply(tasks("b1", "a2"), 1.0)
    work.apply(RESULT, 2.0)
    assert work.running == ["b1", "a2"]
    assert work.holds(10_000.0, 30.0)


def test_after_the_last_task_a_turn_or_a_quiet_window_holds() -> None:
    work = BackgroundWork()
    work.apply(tasks("b1"), 1.0)
    work.apply(tasks(), 5.0)
    assert work.holds(5.5, 1.0)  # the notification turn may still come
    assert not work.holds(6.5, 1.0)
    work.apply(INIT, 7.0)  # the CLI starts the notification turn by itself
    assert work.holds(100.0, 1.0)
    work.apply(RESULT, 101.0)
    assert work.holds(101.5, 1.0)
    assert not work.holds(102.5, 1.0)


def test_ambient_and_hidden_tasks_do_not_count() -> None:
    work = BackgroundWork()
    work.apply(tasks("m1", ambient=True), 1.0)
    work.apply(tasks("m2", skip_transcript=True), 2.0)
    assert work.running == []
    assert not work.holds(100.0, 1.0)


def lifecycle(command: str, state: str) -> dict[str, object]:
    return {"type": "command_lifecycle", "command_uuid": command, "state": state}


def test_an_input_the_cli_has_not_started_yet_is_waiting() -> None:
    # FIX-runner-hold-background: on a resume the CLI first runs a turn by itself for each
    # notification of work the previous process lost (init, then an empty result) while the
    # written input is still queued; that result is no answer to the input.
    work = BackgroundWork()
    work.apply(lifecycle("c1", "queued"), 0.0)
    work.apply(INIT, 0.1)
    work.apply(RESULT, 0.2)
    assert work.input_waiting
    work.apply(lifecycle("c1", "started"), 0.3)
    assert not work.input_waiting
    work.apply(lifecycle("c2", "queued"), 0.4)
    work.apply(lifecycle("c2", "cancelled"), 0.5)
    assert not work.input_waiting
    work.apply({"type": "command_lifecycle", "state": "queued"}, 0.6)  # no command named
    assert not work.input_waiting


def test_malformed_task_lists_change_nothing() -> None:
    work = BackgroundWork()
    work.apply(tasks("b1"), 1.0)
    work.apply({"type": "system", "subtype": "background_tasks_changed", "tasks": "x"}, 2.0)
    work.apply(
        {"type": "system", "subtype": "background_tasks_changed", "tasks": [1, {"x": 1}]}, 3.0
    )
    assert work.running == []
    work.apply(tasks("b2"), 4.0)
    work.apply({"type": "system", "subtype": "background_tasks_changed"}, 5.0)
    assert work.running == ["b2"]
