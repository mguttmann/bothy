# SPDX-License-Identifier: Apache-2.0
r"""The live box from the stream of a turn (docs/spec/09-agents.md 18.4, design note 12).

`LiveBox.apply` maps one stream message to changes of the SDK's run patcher, which throttles
them to ``agent.run_update_min_interval`` and rebases on version conflicts. Read fields
[derived] from the Agent SDK reference that 18.4 cites:

* ``system/init``: ``/model``.
* ``assistant``: the text items of the newest message become ``/last_text_excerpt`` (a leading
  ``TITLE:`` line removed when it was requested); each ``tool_use`` adds a ``tool_call`` step,
  nested under the step of ``parent_tool_use_id``; ``message.usage`` gives
  ``/metrics/context_pct`` once the model's ``contextWindow`` is known.
* ``user``: each ``tool_result`` ends its step ``completed`` or ``failed``; the echo of the
  runner's own prompt is ignored.
* ``system/task_*``: a ``subagent`` step for ``local_agent``, a ``workflow`` step for
  `WORKFLOW_TASK_TYPE`, else a ``background_job`` step; ``ambient`` tasks get none;
  ``completed``, ``failed`` and ``stopped`` end it. The step hangs under the step of the tool
  call its ``tool_use_id`` names, when that step exists (M2-51, D-RV-3); the ``usage`` of
  ``task_progress`` and ``task_notification`` becomes the detail member ``usage`` (``tokens``,
  ``tool_uses``, ``duration_ms``; never a cost, D-RV-5).
* ``tool_progress``: counted; ``/metrics/progress_events`` is patched only when the last change
  of the turn is older than ``runner.progress_patch_interval``, so a long tool is not stalled
  (09 5.5, 18.4).
* ``system/background_tasks_changed``: open background steps outside the set end
  ``completed``; ``/metrics/background_jobs`` is the size of the set.
* ``system/status``: ``compacting`` sets ``/status_line``, its absence removes it.
* ``system/api_retry``: ``/metrics/api_retries`` and ``/metrics/last_api_retry``.

Steps carry no ``detail`` beyond the subagent type and the task usage (data economy) and never
tool input other than the title line. The fields ``tool_use_id``, ``task_type`` (with
`WORKFLOW_TASK_TYPE`) and ``usage`` were checked against the files of the pinned CLI
(``tests/fixtures/recording``, ruling D-M2-51-1).

Examples:
    >>> step_title({"command": "git status\\n--short"})
    'git status'
"""

from __future__ import annotations

from collections.abc import Callable, Mapping
from datetime import datetime
from typing import Any
from uuid import UUID

from bothy_sdk.runs import RunPatcher

from claude_code_runner.title import split_title

__all__ = ["WORKFLOW_TASK_TYPE", "LiveBox", "step_title"]

WORKFLOW_TASK_TYPE = "local_workflow"
"""The ``task_type`` of a workflow task in the pinned Claude Code version (recording of M2-51)."""

_USAGE_FIELDS = (
    ("total_tokens", "tokens"),
    ("tool_uses", "tool_uses"),
    ("duration_ms", "duration_ms"),
)
_TASK_KINDS = ("subagent", "workflow", "background_job")
_TITLE_KEYS = ("command", "file_path", "path", "pattern", "url", "query", "description", "prompt")
_TASK_END = {"completed": "completed", "failed": "failed", "stopped": "cancelled"}
_TASKS = frozenset({"task_started", "task_progress", "task_updated", "task_notification"})


def _first_line(text: str) -> str:
    for line in text.splitlines():
        if line.strip():
            return line.strip()
    return ""


def step_title(tool_input: object) -> str:
    """Return the title of a tool step: the first non-empty line of its main string member.

    Args:
        tool_input: The ``input`` of a ``tool_use`` item.

    Returns:
        The first line of the first string member in the order ``command``, ``file_path``,
        ``path``, ``pattern``, ``url``, ``query``, ``description``, ``prompt``, else of the
        first string member, else the empty string.
    """
    if not isinstance(tool_input, Mapping):
        return ""
    for key in _TITLE_KEYS:
        value = tool_input.get(key)
        if isinstance(value, str) and value.strip():
            return _first_line(value)
    for value in tool_input.values():
        if isinstance(value, str) and value.strip():
            return _first_line(value)
    return ""


def _uuid(value: object) -> UUID | None:
    try:
        return UUID(str(value))
    except ValueError:
        return None


def _items(message: Mapping[str, Any]) -> list[Mapping[str, Any]]:
    inner = message.get("message")
    content = inner.get("content") if isinstance(inner, Mapping) else None
    if not isinstance(content, list):
        return []
    return [i for i in content if isinstance(i, Mapping)]


class _Tracked:
    """The run patcher with a hook on every change handed to it (the progress interval)."""

    def __init__(self, patch: RunPatcher, changed: Callable[[], None]) -> None:
        self._inner = patch
        self._changed = changed

    def __getattr__(self, name: str) -> Any:
        attr = getattr(self._inner, name)
        if not callable(attr):
            return attr

        def call(*args: Any, **kwargs: Any) -> Any:
            self._changed()
            return attr(*args, **kwargs)

        return call


class LiveBox:
    """Maps the stream of one turn to run patches.

    Args:
        patch: The run's patcher (`bothy_sdk.runs.RunHandle.patch`).
        own_uuid: The ``uuid`` of the runner's own prompt line (its echo is ignored).
        title_requested: Whether the system prompt asked for a ``TITLE:`` line.
        context_windows: The ``contextWindow`` per model of the session's last ``result``.
        now: The wall clock for ``finished_at`` and the progress interval.
        progress_interval: ``runner.progress_patch_interval`` in seconds.
    """

    def __init__(  # noqa: PLR0913 - every input of the live box is explicit
        self,
        patch: RunPatcher,
        *,
        own_uuid: str,
        title_requested: bool,
        context_windows: Mapping[str, int],
        now: Callable[[], datetime],
        progress_interval: float = 120.0,
    ) -> None:
        """Create the live box of a turn; see the class docstring."""
        self._patch: Any = _Tracked(patch, self._changed)
        self._progress_interval = progress_interval
        # The run was just started or resumed (progress on the server, 09 5.5): the interval
        # of the first progress patch counts from the creation of the box.
        self._last_change = now()
        self.progress_events = 0
        self._usage: dict[UUID, dict[str, int]] = {}
        self._own = own_uuid
        self._title = title_requested
        self._windows = dict(context_windows)
        self._now = now
        self._model: str | None = None
        self._tools: dict[str, UUID] = {}
        self._tasks: dict[str, tuple[UUID, str]] = {}
        self._ambient: set[str] = set()
        self._ended: set[UUID] = set()
        self.subagents = 0
        self.background_jobs = 0
        self.api_retries = 0
        self.context_pct: float | None = None
        self._status = False
        self.steps_changed = False
        """A step was added or ended since `state` was taken (the turn record keeps the map)."""

    def state(self) -> dict[str, Any]:
        """The step ids and counters for the turn record (09 18.9 crash, design note 12 b).

        Taking it clears `steps_changed`. A runner that adopts the turn passes it to `restore`,
        so a line read twice reuses its step id and a result ends the step its call opened.
        """
        self.steps_changed = False
        return {
            "tools": {k: str(v) for k, v in self._tools.items()},
            "tasks": {k: [str(v), kind] for k, (v, kind) in self._tasks.items()},
            "ambient": sorted(self._ambient),
            "ended": sorted(str(v) for v in self._ended),
            "subagents": self.subagents,
            "background_jobs": self.background_jobs,
            "api_retries": self.api_retries,
            "progress_events": self.progress_events,
        }

    def restore(self, raw: object, known: Mapping[str, str]) -> None:
        """Take over the step ids and counters of `state` from a turn record.

        The record keeps a step id before its patch goes out, so only the steps the server
        has are taken over: a step it lacks is added again when its line is read again, and
        an end it lacks is sent again when the result is read again.

        Args:
            raw: The stored state; anything malformed is skipped.
            known: The status of each step of the server's run, by step id.
        """
        if not isinstance(raw, Mapping):
            return
        tools, tasks = raw.get("tools"), raw.get("tasks")
        if isinstance(tools, Mapping):
            for key, value in tools.items():
                step = _uuid(value)
                if isinstance(key, str) and step is not None and str(step) in known:
                    self._tools[key] = step
        if isinstance(tasks, Mapping):
            for key, value in tasks.items():
                if isinstance(key, str) and isinstance(value, list) and len(value) == 2:
                    step = _uuid(value[0])
                    if step is not None and str(step) in known and value[1] in _TASK_KINDS:
                        self._tasks[key] = (step, str(value[1]))
        ambient, ended = raw.get("ambient"), raw.get("ended")
        if isinstance(ambient, list):
            self._ambient.update(a for a in ambient if isinstance(a, str))
        if isinstance(ended, list):
            self._ended.update(
                e
                for e in map(_uuid, ended)
                if e is not None and known.get(str(e), "running") not in ("running", "pending")
            )
        for name in ("subagents", "background_jobs", "api_retries", "progress_events"):
            value = raw.get(name)
            if isinstance(value, int) and not isinstance(value, bool) and value >= 0:
                setattr(self, name, value)

    def apply(self, message: Mapping[str, Any]) -> None:
        """Apply one stream message.

        Args:
            message: A decoded message of the stream reader.
        """
        kind = message.get("type")
        if kind == "assistant":
            self._assistant(message)
        elif kind == "user":
            self._user(message)
        elif kind == "system":
            self._system(message)
        elif kind == "tool_progress":
            self._progress()

    def _changed(self) -> None:
        self._last_change = self._now()

    def _progress(self) -> None:
        # No visible change; a patch only when the last one of the turn is older than
        # runner.progress_patch_interval, so the server sees progress against run.stall_after
        # (09 5.5, 18.4) without a patch per message.
        self.progress_events += 1
        if (self._now() - self._last_change).total_seconds() >= self._progress_interval:
            self._patch.set_metric("progress_events", self.progress_events)

    # ---- assistant and user ---------------------------------------------------------------

    def _assistant(self, message: Mapping[str, Any]) -> None:
        items = _items(message)
        texts = [
            i["text"] for i in items if i.get("type") == "text" and isinstance(i.get("text"), str)
        ]
        if texts:
            _, excerpt = split_title("\n".join(texts), self._title)
            if excerpt:
                self._patch.set_last_text_excerpt(excerpt)
        parent_ref = message.get("parent_tool_use_id")
        parent = self._tools.get(parent_ref) if isinstance(parent_ref, str) else None
        for item in items:
            if item.get("type") != "tool_use" or not isinstance(item.get("id"), str):
                continue
            if item["id"] in self._tools:
                continue  # read again after an adoption: the step exists
            name = item.get("name") if isinstance(item.get("name"), str) else "tool"
            self._tools[item["id"]] = self._patch.add_step(
                "tool_call",
                str(name),
                title=step_title(item.get("input")),
                status="running",
                parent_step_id=parent,
            )
            self.steps_changed = True
        inner = message.get("message")
        model = inner.get("model") if isinstance(inner, Mapping) else None
        if isinstance(model, str):
            self._model = self._model or model
        usage = inner.get("usage") if isinstance(inner, Mapping) else None
        if isinstance(usage, Mapping):
            self._context(usage)

    def _context(self, usage: Mapping[str, Any]) -> None:
        window = self._windows.get(self._model or "")
        if not window:
            return
        used = sum(
            v
            for k in ("input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens")
            if isinstance(v := usage.get(k), int) and not isinstance(v, bool)
        )
        self.context_pct = round(min(100.0, 100.0 * used / window), 1)
        self._patch.set_metric("context_pct", self.context_pct)

    def _user(self, message: Mapping[str, Any]) -> None:
        if message.get("uuid") == self._own or message.get("isReplay") is True:
            # The echo of a stdin line: the prompt's is ignored, a follow-up's sets absorbed
            # (claude_code_runner.followups); never a tool result (M2-35 note 4 b).
            return
        for item in _items(message):
            if item.get("type") != "tool_result":
                continue
            step = self._tools.get(str(item.get("tool_use_id")))
            if step is None or step in self._ended:
                continue
            self._ended.add(step)
            self.steps_changed = True
            status = "failed" if item.get("is_error") is True else "completed"
            self._patch.update_step(step, status=status, finished_at=self._now())

    # ---- system ---------------------------------------------------------------------------

    def _system(self, message: Mapping[str, Any]) -> None:
        subtype = message.get("subtype")
        if subtype == "init":
            model = message.get("model")
            if isinstance(model, str) and model:
                self._model = model
                self._patch.set_model(model)
        elif subtype in _TASKS:
            self._task(message)
        elif subtype == "background_tasks_changed":
            self._background(message)
        elif subtype == "status":
            self._status_line(message)
        elif subtype == "api_retry":
            self._retry(message)

    def _task(self, message: Mapping[str, Any]) -> None:
        task_id = message.get("task_id")
        if not isinstance(task_id, str):
            return
        if message.get("ambient") is True and task_id not in self._ambient:
            self._ambient.add(task_id)
            self.steps_changed = True
        if task_id in self._ambient:
            return
        if task_id not in self._tasks:
            self._add_task(task_id, message)
        self._task_usage(task_id, message)
        end = _TASK_END.get(str(message.get("status")))
        if end is not None:
            self._end(self._tasks[task_id][0], end)

    def _add_task(self, task_id: str, message: Mapping[str, Any]) -> None:
        description = message.get("description")
        title = _first_line(description) if isinstance(description, str) else ""
        # The step of the tool call that started the task (D-RV-3); the tool call comes first in
        # the stream, so a miss means the task has no such call and stays at the root.
        ref = message.get("tool_use_id")
        parent = self._tools.get(ref) if isinstance(ref, str) else None
        task_type = message.get("task_type")
        if task_type == "local_agent":
            step = self._patch.add_step(
                "subagent",
                "local_agent",
                title=title,
                detail={"type": "local_agent"},
                parent_step_id=parent,
            )
            self._tasks[task_id] = (step, "subagent")
            self.subagents += 1
            self._patch.set_metric("subagents", self.subagents)
        else:
            kind = "workflow" if task_type == WORKFLOW_TASK_TYPE else "background_job"
            name = task_type if isinstance(task_type, str) and task_type else "background_job"
            step = self._patch.add_step(kind, name, title=title, parent_step_id=parent)
            self._tasks[task_id] = (step, kind)
        self.steps_changed = True

    def _task_usage(self, task_id: str, message: Mapping[str, Any]) -> None:
        # The usage of task_progress and task_notification becomes detail.usage (09 5.6); the
        # whole detail is replaced, so a subagent keeps its type. No cost: the runner never
        # estimates one (D-RV-5).
        raw = message.get("usage")
        if not isinstance(raw, Mapping):
            return
        usage = {
            out: value
            for key, out in _USAGE_FIELDS
            if isinstance(value := raw.get(key), int) and not isinstance(value, bool) and value >= 0
        }
        step, kind = self._tasks[task_id]
        if not usage or self._usage.get(step) == usage:
            return
        self._usage[step] = usage
        detail: dict[str, Any] = {"type": "local_agent"} if kind == "subagent" else {}
        detail["usage"] = usage
        self._patch.update_step(step, detail=detail)

    def _end(self, step: UUID, status: str) -> None:
        if step in self._ended:
            return
        self._ended.add(step)
        self.steps_changed = True
        self._patch.update_step(step, status=status, finished_at=self._now())

    def _background(self, message: Mapping[str, Any]) -> None:
        tasks = message.get("tasks")
        if not isinstance(tasks, list):
            return
        current = {t.get("task_id") for t in tasks if isinstance(t, Mapping)}
        for task_id, (step, kind) in self._tasks.items():
            if kind == "background_job" and task_id not in current:
                self._end(step, "completed")
        self.background_jobs = len(current)
        self._patch.set_metric("background_jobs", self.background_jobs)

    def _status_line(self, message: Mapping[str, Any]) -> None:
        if message.get("status") == "compacting":
            self._status = True
            self._patch.set_status_line("compacting")
        elif self._status:
            self._status = False
            self._patch.clear_status_line()

    def _retry(self, message: Mapping[str, Any]) -> None:
        self.api_retries += 1
        self._patch.set_metric("api_retries", self.api_retries)
        attempt = message.get("attempt")
        kwargs: dict[str, Any] = {}
        for key in ("max_retries", "retry_delay_ms"):
            value = message.get(key)
            if isinstance(value, int) and not isinstance(value, bool):
                kwargs[key] = value
        error = message.get("error")
        if isinstance(error, str):
            kwargs["error"] = error
        attempt_no = (
            attempt
            if isinstance(attempt, int) and not isinstance(attempt, bool)
            else self.api_retries
        )
        self._patch.set_last_api_retry(attempt_no, **kwargs)
