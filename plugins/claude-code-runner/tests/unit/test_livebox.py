# SPDX-License-Identifier: Apache-2.0
"""The live box from the stream, one case per row of design note 12 (09 18.4)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any
from uuid import UUID, uuid4

from claude_code_runner.livebox import LiveBox, step_title

NOW = datetime(2026, 10, 21, 9, 0, tzinfo=UTC)
OWN = "0192d3a0-0000-4000-8000-0000000000aa"


class Patch:
    """Records the RunPatcher calls the live box makes."""

    def __init__(self) -> None:
        self.values: dict[str, Any] = {}
        self.steps: dict[UUID, dict[str, Any]] = {}
        self.calls: list[str] = []

    def set_model(self, model: str) -> None:
        self.values["/model"] = model

    def set_last_text_excerpt(self, text: str) -> None:
        self.values["/last_text_excerpt"] = text

    def set_metric(self, name: str, value: float) -> None:
        self.values[f"/metrics/{name}"] = value

    def set_last_api_retry(self, attempt: int, **kw: Any) -> None:
        self.values["/metrics/last_api_retry"] = {"attempt": attempt, **kw}

    def set_status_line(self, text: str) -> None:
        self.values["/status_line"] = text

    def clear_status_line(self) -> None:
        self.calls.append("clear_status_line")
        self.values.pop("/status_line", None)

    def add_step(self, kind: str, name: str, **kw: Any) -> UUID:
        step_id = uuid4()
        self.steps[step_id] = {"kind": kind, "name": name, **kw}
        return step_id

    def update_step(self, step_id: UUID, **kw: Any) -> None:
        self.steps[step_id].update({k: v for k, v in kw.items() if v is not None})


def box(title: bool = False, windows: dict[str, int] | None = None) -> tuple[LiveBox, Patch]:
    patch = Patch()
    live = LiveBox(
        patch,  # type: ignore[arg-type]
        own_uuid=OWN,
        title_requested=title,
        context_windows=windows or {},
        now=lambda: NOW,
    )
    return live, patch


def assistant(
    content: list[dict[str, Any]], parent: str | None = None, usage: Any = None
) -> dict[str, Any]:
    msg: dict[str, Any] = {"type": "assistant", "message": {"content": content}}
    if parent:
        msg["parent_tool_use_id"] = parent
    if usage:
        msg["message"]["usage"] = usage
    return msg


def test_init_sets_model() -> None:
    live, patch = box()
    live.apply({"type": "system", "subtype": "init", "model": "model-large", "session_id": "x"})
    assert patch.values["/model"] == "model-large"


def test_text_excerpt_and_title_stripped() -> None:
    live, patch = box(title=True)
    live.apply(assistant([{"type": "text", "text": "TITLE: Fix it\nWorking on it."}]))
    assert patch.values["/last_text_excerpt"] == "Working on it."
    live2, patch2 = box(title=False)
    live2.apply(assistant([{"type": "text", "text": "TITLE: Fix it\nWorking."}]))
    assert patch2.values["/last_text_excerpt"] == "TITLE: Fix it\nWorking."


def test_tool_use_and_result_nested() -> None:
    live, patch = box()
    live.apply(
        assistant(
            [
                {
                    "type": "tool_use",
                    "id": "tu1",
                    "name": "Task",
                    "input": {"description": "Explore\nmore"},
                }
            ]
        )
    )
    live.apply(
        assistant(
            [{"type": "tool_use", "id": "tu2", "name": "Bash", "input": {"command": "git status"}}],
            parent="tu1",
        )
    )
    (outer, outer_step), (inner, inner_step) = list(patch.steps.items())
    assert outer_step == {
        "kind": "tool_call",
        "name": "Task",
        "title": "Explore",
        "status": "running",
        "parent_step_id": None,
    }
    assert inner_step["parent_step_id"] == outer
    assert inner_step["title"] == "git status"
    live.apply(
        {
            "type": "user",
            "message": {
                "content": [{"type": "tool_result", "tool_use_id": "tu2", "is_error": True}]
            },
        }
    )
    live.apply(
        {"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "tu1"}]}}
    )
    assert patch.steps[inner]["status"] == "failed"
    assert patch.steps[outer]["status"] == "completed"
    assert patch.steps[outer]["finished_at"] == NOW


def test_own_echo_ignored_and_tool_progress_no_change() -> None:
    live, patch = box()
    live.apply(
        {
            "type": "user",
            "isReplay": True,
            "uuid": OWN,
            "message": {"content": [{"type": "text", "text": "x"}]},
        }
    )
    live.apply({"type": "tool_progress", "tool_use_id": "a"})
    assert patch.values == {}
    assert patch.steps == {}


def test_tasks_ambient_and_background() -> None:
    live, patch = box()
    live.apply(
        {
            "type": "system",
            "subtype": "task_started",
            "task_id": "a1",
            "description": "Review",
            "task_type": "local_agent",
        }
    )
    live.apply(
        {
            "type": "system",
            "subtype": "task_started",
            "task_id": "b1",
            "description": "npm test",
            "task_type": "local_bash",
        }
    )
    live.apply(
        {
            "type": "system",
            "subtype": "task_started",
            "task_id": "c1",
            "description": "hidden",
            "task_type": "local_agent",
            "ambient": True,
        }
    )
    kinds = sorted((s["kind"], s.get("detail")) for s in patch.steps.values())
    assert kinds == [("background_job", None), ("subagent", {"type": "local_agent"})]
    assert live.subagents == 1
    assert patch.values["/metrics/subagents"] == 1
    live.apply(
        {"type": "system", "subtype": "task_notification", "task_id": "a1", "status": "stopped"}
    )
    sub = next(s for s in patch.steps.values() if s["kind"] == "subagent")
    assert sub["status"] == "cancelled"
    live.apply(
        {
            "type": "system",
            "subtype": "background_tasks_changed",
            "tasks": [{"task_id": "x9", "description": "other"}],
        }
    )
    bg = next(s for s in patch.steps.values() if s["kind"] == "background_job")
    assert bg["status"] == "completed"
    assert patch.values["/metrics/background_jobs"] == 1
    assert live.background_jobs == 1


def test_status_compacting_and_retry() -> None:
    live, patch = box()
    live.apply({"type": "system", "subtype": "status", "status": "compacting"})
    assert patch.values["/status_line"] == "compacting"
    live.apply({"type": "system", "subtype": "status", "status": None})
    assert "clear_status_line" in patch.calls
    live.apply(
        {
            "type": "system",
            "subtype": "api_retry",
            "attempt": 2,
            "max_retries": 10,
            "retry_delay_ms": 500,
            "error": "overloaded",
        }
    )
    assert patch.values["/metrics/api_retries"] == 1
    assert patch.values["/metrics/last_api_retry"] == {
        "attempt": 2,
        "max_retries": 10,
        "retry_delay_ms": 500,
        "error": "overloaded",
    }
    assert live.api_retries == 1


def test_context_pct_only_with_known_window() -> None:
    usage = {
        "input_tokens": 1000,
        "cache_read_input_tokens": 500,
        "cache_creation_input_tokens": 500,
    }
    live, patch = box()
    live.apply({"type": "system", "subtype": "init", "model": "model-large"})
    live.apply(assistant([{"type": "text", "text": "a"}], usage=usage))
    assert "/metrics/context_pct" not in patch.values
    assert live.context_pct is None
    live2, patch2 = box(windows={"model-large": 200000})
    live2.apply({"type": "system", "subtype": "init", "model": "model-large"})
    live2.apply(assistant([{"type": "text", "text": "a"}], usage=usage))
    assert patch2.values["/metrics/context_pct"] == 1.0


def test_step_title_order() -> None:
    assert step_title({"query": "q", "command": "  \nls -la\nmore"}) == "ls -la"
    assert step_title({"pattern": "*.py", "path": "src"}) == "src"
    assert step_title({"other": "first", "z": "second"}) == "first"
    assert step_title({"n": 1}) == ""
    assert step_title("not a mapping") == ""


def test_unknown_messages_ignored() -> None:
    live, patch = box()
    live.apply({"type": "system", "subtype": "hook_response"})
    live.apply({"type": "assistant"})
    live.apply({"type": "user", "message": {"content": "plain"}})
    assert patch.values == {}


def test_restore_keeps_only_the_steps_the_server_has() -> None:
    # A step id the record kept but whose add never reached the server is dropped, so the line
    # read again adds it; an end the server never got is dropped, so the result read again ends
    # it. A step the server has keeps its id (09 18.9 crash, design note 12 d).
    def call(tool_id: str) -> dict[str, Any]:
        return assistant([{"type": "tool_use", "id": tool_id, "name": "Bash", "input": {}}])

    def result(tool_id: str) -> dict[str, Any]:
        content = [{"type": "tool_result", "tool_use_id": tool_id}]
        return {"type": "user", "message": {"content": content}}

    first, before = box()
    for message in (call("open"), call("lost"), call("done"), result("done"), call("unended")):
        first.apply(message)
    first.apply(result("unended"))
    open_, _, done, unended = list(before.steps)
    known = {str(open_): "running", str(done): "completed", str(unended): "running"}
    adopted, after = box()
    after.steps = {open_: {}, done: {}, unended: {}}
    adopted.restore(first.state(), known)
    for tool_id in ("open", "lost", "done", "unended"):
        adopted.apply(call(tool_id))
        adopted.apply(result(tool_id))
    added = [s for s in after.steps if s not in (open_, done, unended)]
    assert len(added) == 1
    assert after.steps[added[0]]["status"] == "completed"
    assert after.steps[open_]["status"] == "completed"
    assert after.steps[unended]["status"] == "completed"
    assert after.steps[done] == {}


def test_keepalive_while_background_work_runs_reports_progress_once_per_interval() -> None:
    # Between turns the stream can stay silent while background work runs; the run must not
    # look stalled (09 5.5, 18.5), so the box reports progress once per interval.
    clock = [NOW]
    patch = Patch()
    live = LiveBox(
        patch,  # type: ignore[arg-type]
        own_uuid=OWN,
        title_requested=False,
        context_windows={},
        now=lambda: clock[0],
        progress_interval=60.0,
    )
    live.keepalive()
    assert patch.values == {}
    clock[0] = NOW + timedelta(seconds=61)
    live.keepalive()
    assert patch.values == {"/metrics/progress_events": 1}
    clock[0] = NOW + timedelta(seconds=90)
    live.keepalive()
    assert patch.values == {"/metrics/progress_events": 1}
    clock[0] = NOW + timedelta(seconds=122)
    live.keepalive()
    assert patch.values == {"/metrics/progress_events": 2}
