# SPDX-License-Identifier: Apache-2.0
"""Run helpers: the live-box patcher and the handle of a running run (11 11.2 row ``runs``).

`RunPatcher` keeps the desired state per path of docs/spec/09-agents.md 5.3 and sends at most
one patch per ``agent.run_update_min_interval``; a later value on the same path replaces the
earlier one, and an `add_step` followed by updates goes out as one ``add`` (docs/DECISIONS.md
D-M2-29-9). Its transport is REST ``patchRun``, because the answer carries the new version and a
``409 base_version_conflict`` carries the current document as the new base; the gateway op
``run.update`` is offered through `RunPatcher.use_gateway`. A ``409 conflict`` with ``reason``
``run_not_active`` or a ``403 invocation_token_invalid`` with ``reason`` ``run_inactive`` stops the
patcher and ends the work on the run.

Examples:
    >>> any(p.pattern == "/status_line" for p in PATCH_PATHS)
    True
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import re
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime
from typing import Any
from uuid import UUID

import httpx

from bothy_sdk.client import Client, expect
from bothy_sdk.clock import Clock
from bothy_sdk.errors import (
    BaseVersionConflict,
    ConflictError,
    GatewayOpError,
    InvocationTokenInvalid,
    ProblemError,
    RateLimitedError,
    ServerError,
)
from bothy_sdk.generated.api.agent_runtime import await_run_input, finish_run, patch_run
from bothy_sdk.generated.api.runs import get_run, get_run_context
from bothy_sdk.generated.models import (
    CostReport,
    JsonPatchOperation,
    JsonPatchOperationOp,
    Run,
    RunAwaitInput,
    RunContext,
    RunFinish,
    RunFinishState,
    RunMetrics,
    RunPatch,
    RunPatchResult,
)
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings

__all__ = [
    "METRIC_FIELDS",
    "PATCH_PATHS",
    "PatchPath",
    "RunHandle",
    "RunPatcher",
    "is_run_ended",
]

_LOG = logging.getLogger("bothy_sdk")

METRIC_FIELDS = (
    "context_pct",
    "tokens_in",
    "tokens_out",
    "tokens_cache_read",
    "tokens_cache_write",
    "tokens_cache_write_1h",
    "subagents",
    "background_jobs",
    "api_retries",
)
"""Number members of ``run_metrics.v1`` an adapter may set (``/metrics/<field>``)."""

_STEP_FIELDS = ("status", "title", "detail", "finished_at")
_FOLLOWUP_ORDER = {"delivered": 1, "absorbed": 2}


@dataclass(frozen=True)
class PatchPath:
    """One row of the path table of docs/spec/09-agents.md 5.3.

    Attributes:
        pattern: The path with placeholders ``<field>``, ``<id>`` and ``<message_id>``.
        ops: The RFC 6902 operations allowed on it.
    """

    pattern: str
    ops: frozenset[str]

    def matches(self, path: str) -> bool:
        """Tell whether a concrete path is this row.

        Args:
            path: A JSON pointer such as ``/steps/<uuid>/status``.

        Returns:
            True for a match.
        """
        return _compile(self.pattern).fullmatch(path) is not None


def _compile(pattern: str) -> re.Pattern[str]:
    parts = []
    for segment in pattern.split("/"):
        if segment == "<field>":
            parts.append("(?:" + "|".join(METRIC_FIELDS) + ")")
        elif segment.startswith("<"):
            parts.append("[0-9a-fA-F-]{36}")
        else:
            parts.append(re.escape(segment))
    return re.compile("/".join(parts))


_ADD_REPLACE = frozenset({"add", "replace"})
_ALL = frozenset({"add", "replace", "remove"})
_REPLACE = frozenset({"replace"})

PATCH_PATHS: tuple[PatchPath, ...] = (
    PatchPath("/status_line", _ALL),
    PatchPath("/last_text_excerpt", _ADD_REPLACE),
    PatchPath("/model", _REPLACE),
    PatchPath("/metrics/<field>", _ADD_REPLACE),
    PatchPath("/metrics/last_api_retry", _ALL),
    PatchPath("/cost", _REPLACE),
    PatchPath("/plan", _ALL),
    PatchPath("/steps/<id>", frozenset({"add"})),
    *(PatchPath(f"/steps/<id>/{f}", _REPLACE) for f in _STEP_FIELDS),
    PatchPath("/followups/<message_id>/state", _REPLACE),
)
"""The paths an adapter may patch, with their operations (09 5.3); a test holds it equal."""


def _allowed(op: str, path: str) -> bool:
    return any(p.matches(path) and op in p.ops for p in PATCH_PATHS)


def is_run_ended(error: ProblemError) -> bool:
    """Tell whether an error says the run has ended (``run_not_active`` or ``run_inactive``).

    Args:
        error: A problem of a run operation.

    Returns:
        True when the work on the run must end.
    """
    if isinstance(error, ConflictError) and error.code == "conflict":
        return error.reason == "run_not_active"
    return isinstance(error, InvocationTokenInvalid) and error.reason == "run_inactive"


def _transient(error: ProblemError) -> bool:
    """A refusal worth sending the same changes again: 5xx, 429 or an exhausted rebase."""
    return isinstance(error, ServerError | RateLimitedError | BaseVersionConflict)


def _iso(value: datetime) -> str:
    return value.isoformat().replace("+00:00", "Z")


class RunPatcher:
    """Collects live-box changes of one run and sends them throttled.

    Args:
        client: The REST client (bound to the run by the caller, 07 SR9).
        run_id: The run.
        base_version: The version of the ``Run`` that ``startRun`` or ``resumeRun`` answered.
        settings: ``agent.run_update_min_interval`` and ``sdk.retry_max_attempts``.
        clock: The time source of the throttle.
        on_ended: Called once when the server says the run has ended.
    """

    def __init__(  # noqa: PLR0913 - every input of the patcher is explicit
        self,
        client: Client,
        run_id: UUID,
        base_version: int,
        *,
        settings: SdkSettings | None = None,
        clock: Clock | None = None,
        on_ended: Callable[[], None] | None = None,
    ) -> None:
        """Create the patcher; see the class docstring."""
        self.client = client
        self.run_id = run_id
        self.base_version = base_version
        self.settings = settings or client.settings
        self.clock = clock or client.clock
        self._on_ended = on_ended
        self._pending: dict[str, tuple[str, Any]] = {}
        self._new_steps: set[UUID] = set()
        self._followups: dict[UUID, str] = {}
        self._last_sent: float | None = None
        self._timer: asyncio.Task[None] | None = None
        self._lock = asyncio.Lock()
        self._gateway: Any = None
        self._gateway_refs = 0
        self.ended = False
        self.patches_sent = 0

    # ---- setters -------------------------------------------------------------------------

    def _set(self, path: str, op: str, value: Any = None) -> None:
        if self.ended:
            return
        self._pending[path] = (op, value)
        self._schedule()

    def set_status_line(self, text: str) -> None:
        """Set ``/status_line``.

        Args:
            text: The status line.
        """
        self._set("/status_line", "replace", text)

    def clear_status_line(self) -> None:
        """Remove ``/status_line``."""
        self._set("/status_line", "remove")

    def set_last_text_excerpt(self, text: str) -> None:
        """Set ``/last_text_excerpt`` (the server keeps its last ``run.excerpt_max_chars``).

        Args:
            text: The draft text so far.
        """
        self._set("/last_text_excerpt", "replace", text)

    def set_model(self, model: str) -> None:
        """Set ``/model``.

        Args:
            model: The model name of the runtime.
        """
        self._set("/model", "replace", model)

    def set_metric(self, name: str, value: float) -> None:
        """Set ``/metrics/<name>``.

        Args:
            name: One of `METRIC_FIELDS`.
            value: The number.

        Raises:
            ValueError: ``name`` is not a metric of ``run_metrics.v1``.
        """
        if name not in METRIC_FIELDS:
            raise ValueError(f"unknown metric: {name}")
        self._set(f"/metrics/{name}", "replace", value)

    def set_last_api_retry(
        self,
        attempt: int,
        *,
        max_retries: int | None = None,
        retry_delay_ms: int | None = None,
        error: str | None = None,
    ) -> None:
        """Set ``/metrics/last_api_retry``.

        Args:
            attempt: The attempt number.
            max_retries: The runtime's limit, if known.
            retry_delay_ms: The wait before the next attempt, if known.
            error: A short error text, if any.
        """
        value: dict[str, Any] = {"attempt": attempt}
        if max_retries is not None:
            value["max_retries"] = max_retries
        if retry_delay_ms is not None:
            value["retry_delay_ms"] = retry_delay_ms
        if error is not None:
            value["error"] = error
        self._set("/metrics/last_api_retry", "replace", value)

    def clear_last_api_retry(self) -> None:
        """Remove ``/metrics/last_api_retry``."""
        self._set("/metrics/last_api_retry", "remove")

    def set_cost(self, amount_micros: int, currency: str) -> None:
        """Set ``/cost``; always an estimate (``is_estimate: true``).

        Args:
            amount_micros: The amount in millionths of the currency unit.
            currency: The ISO 4217 code.
        """
        self._set(
            "/cost",
            "replace",
            {"amount_micros": amount_micros, "currency": currency, "is_estimate": True},
        )

    def set_plan(self, items: Sequence[Mapping[str, str]]) -> None:
        """Set ``/plan``.

        Args:
            items: Plan items with ``id``, ``text`` and ``status``.
        """
        self._set("/plan", "replace", {"items": [dict(i) for i in items]})

    def clear_plan(self) -> None:
        """Remove ``/plan``."""
        self._set("/plan", "remove")

    def add_step(  # noqa: PLR0913 - the members of a step
        self,
        kind: str,
        name: str,
        *,
        title: str = "",
        status: str = "running",
        parent_step_id: UUID | None = None,
        detail: Mapping[str, Any] | None = None,
    ) -> UUID:
        """Add a step; its ID is a UUIDv7 of the adapter, the server assigns ``ordinal``.

        Args:
            kind: ``tool_call``, ``subagent``, ``background_job``, ``workflow`` or ``status``.
            name: The step name.
            title: The display title.
            status: The step status.
            parent_step_id: The parent step, if any.
            detail: A JSON object with details.

        Returns:
            The step ID.
        """
        step_id = uuid7()
        value: dict[str, Any] = {
            "id": str(step_id),
            "kind": kind,
            "name": name,
            "title": title,
            "status": status,
        }
        if parent_step_id is not None:
            value["parent_step_id"] = str(parent_step_id)
        if detail is not None:
            value["detail"] = dict(detail)
        self._new_steps.add(step_id)
        self._set(f"/steps/{step_id}", "add", value)
        return step_id

    def update_step(
        self,
        step_id: UUID,
        *,
        status: str | None = None,
        title: str | None = None,
        detail: Mapping[str, Any] | None = None,
        finished_at: datetime | None = None,
    ) -> None:
        """Change a step; an unsent ``add`` absorbs the change.

        Args:
            step_id: The step.
            status: The new status.
            title: The new title.
            detail: The new detail object.
            finished_at: The end time.
        """
        changes: dict[str, Any] = {}
        if status is not None:
            changes["status"] = status
        if title is not None:
            changes["title"] = title
        if detail is not None:
            changes["detail"] = dict(detail)
        if finished_at is not None:
            changes["finished_at"] = _iso(finished_at)
        add_path = f"/steps/{step_id}"
        pending_add = self._pending.get(add_path)
        if pending_add is not None and pending_add[0] == "add":
            pending_add[1].update(changes)
            self._schedule()
            return
        for name, value in changes.items():
            self._set(f"{add_path}/{name}", "replace", value)

    def _followup(self, message_id: UUID, state: str) -> None:
        current = self._followups.get(message_id)
        if current is not None and _FOLLOWUP_ORDER[current] >= _FOLLOWUP_ORDER[state]:
            raise ValueError("follow-up states only move forward")
        self._followups[message_id] = state
        self._set(f"/followups/{message_id}/state", "replace", state)

    def followup_delivered(self, message_id: UUID) -> None:
        """Mark a follow-up as delivered to the runtime.

        Args:
            message_id: The follow-up message.

        Raises:
            ValueError: The follow-up is already further.
        """
        self._followup(message_id, "delivered")

    def followup_absorbed(self, message_id: UUID) -> None:
        """Mark a follow-up as absorbed by the runtime.

        Args:
            message_id: The follow-up message.

        Raises:
            ValueError: The follow-up is already absorbed.
        """
        self._followup(message_id, "absorbed")

    # ---- sending -------------------------------------------------------------------------

    def use_gateway(self, gateway: Any) -> None:
        """Send patches with the gateway op ``run.update`` instead of REST.

        The gateway has no success reply, so the next base is assumed as ``base_version + 1``;
        after an ``error`` frame with ``base_version_conflict`` route it to `on_gateway_error`,
        which loads ``getRun`` and sends again.

        Args:
            gateway: A `bothy_sdk.gateway.Gateway`.
        """
        self._gateway = gateway

    def _schedule(self) -> None:
        if self._timer is not None and not self._timer.done():
            return
        with contextlib.suppress(RuntimeError):
            self._timer = asyncio.get_running_loop().create_task(self._flush_later())

    async def _flush_later(self) -> None:
        if self._last_sent is not None:
            wait = self._last_sent + self.settings.run_update_min_interval - self.clock.monotonic()
            if wait > 0:
                await self.clock.sleep(wait)
        try:
            await self.flush()
        except ProblemError as exc:
            _LOG.info("run %s: patch failed code=%s", self.run_id, exc.code)
        except httpx.TransportError as exc:
            _LOG.info("run %s: patch failed: %s", self.run_id, type(exc).__name__)

    def _ops(self) -> list[JsonPatchOperation]:
        ops = []
        for path, (op, value) in self._pending.items():
            item = JsonPatchOperation(op=JsonPatchOperationOp(op), path=path)
            if op != "remove":
                item.value = value
            ops.append(item)
        return ops

    async def flush(self) -> None:
        """Send the pending changes now (one patch), rebasing after a version conflict.

        Raises:
            ProblemError: A refusal other than the end of the run, or the version conflict
                after ``sdk.retry_max_attempts`` rounds.
        """
        async with self._lock:
            if self.ended or not self._pending:
                return
            ops = self._ops()
            sent = dict(self._pending)
            self._pending.clear()
            try:
                await self._send(ops)
            except ProblemError as exc:
                if is_run_ended(exc):
                    self._end()
                    return
                if _transient(exc):
                    self._restore(sent)
                # A permanent refusal (for example 400 validation_failed) drops the patch, so
                # one bad value never blocks the later updates of the run.
                raise
            except httpx.TransportError:
                self._restore(sent)
                raise

    def _restore(self, sent: dict[str, tuple[str, Any]]) -> None:
        # Values set while the patch was on its way are newer and win.
        for path, change in sent.items():
            self._pending.setdefault(path, change)

    async def send_ops(self, ops: Sequence[JsonPatchOperation]) -> int:
        """Send raw RFC 6902 operations at once, after checking them against `PATCH_PATHS`.

        Args:
            ops: The operations.

        Returns:
            The new version.

        Raises:
            ValueError: An operation names a path or op outside 09 5.3 (before any request).
            ProblemError: The server refused.
        """
        for item in ops:
            if not _allowed(str(item.op.value), item.path):
                raise ValueError(f"patch path not allowed: {item.op.value} {item.path}")
        async with self._lock:
            return await self._send(list(ops))

    async def _send(self, ops: list[JsonPatchOperation]) -> int:
        if self._gateway is not None:
            return await self._send_gateway(ops)
        rounds = max(1, self.settings.retry_max_attempts)
        for attempt in range(rounds):
            try:
                result = await self.client.call(
                    patch_run.asyncio_detailed,
                    run_id=self.run_id,
                    body=RunPatch(base_version=self.base_version, ops=ops),
                )
            except BaseVersionConflict as exc:
                current = exc.run
                if current is None or attempt == rounds - 1:
                    raise
                self.base_version = current.version
                continue
            self.base_version = expect(result, RunPatchResult).version
            self._last_sent = self.clock.monotonic()
            self.patches_sent += 1
            return self.base_version
        raise AssertionError("unreachable")  # pragma: no cover

    async def _send_gateway(self, ops: list[JsonPatchOperation]) -> int:
        self._gateway_refs += 1
        self._sent_ops = ops
        await self._gateway.send_run_update(
            self.run_id, self.base_version, ops, ref=f"{self.run_id}:{self._gateway_refs}"
        )
        self.base_version += 1
        self._last_sent = self.clock.monotonic()
        self.patches_sent += 1
        return self.base_version

    async def on_gateway_error(self, error: GatewayOpError) -> None:
        """Handle an ``error`` frame of a ``run.update`` of this run.

        ``base_version_conflict`` loads ``getRun`` and sends the last ops again on its version;
        ``run_not_active`` and ``run_inactive`` end the patcher.

        Args:
            error: The error of the frame.
        """
        if error.code == "base_version_conflict":
            loaded = await self.client.call(get_run.asyncio_detailed, run_id=self.run_id)
            run = expect(loaded, Run)
            self.base_version = run.version
            ops = getattr(self, "_sent_ops", [])
            if ops:
                async with self._lock:
                    await self._send_gateway(ops)
        elif error.code == "conflict" or (error.detail or "").endswith("run_not_active"):
            self._end()

    def _end(self) -> None:
        if self.ended:
            return
        self.ended = True
        self._pending.clear()
        _LOG.info("run %s ended on the server; patcher stopped", self.run_id)
        if self._on_ended is not None:
            self._on_ended()

    async def aclose(self) -> None:
        """Stop the throttle timer without sending."""
        if self._timer is not None:
            self._timer.cancel()
            with contextlib.suppress(asyncio.CancelledError, ProblemError):
                await self._timer


class RunHandle:
    """What a handler gets for one run: patches, context, input, the end and replies.

    The adapter creates it before ``prepare`` (the run is still ``queued``) and starts the
    patcher once ``startRun`` or ``resumeRun`` answered; inside the handler every request of the
    client is bound to the run (07 SR9).

    Attributes:
        run_id: The run.
        thread_id: The run's thread.
        token_jti: The ``jti`` of the token the run started or resumed with (``None`` before).
        outbox: The outbox for replies (`bothy_sdk.outbox.Outbox`).
        cancelled: Set when the run was cancelled or ended on the server.
        finished: True once `finish` ran.
        waiting: True while the run waits for input.
    """

    def __init__(  # noqa: PLR0913 - the parts of a run
        self,
        client: Client,
        run_id: UUID,
        thread_id: UUID,
        *,
        outbox: Any,
        settings: SdkSettings | None = None,
        on_waiting: Callable[[], Awaitable[None]] | None = None,
    ) -> None:
        """Create the handle of a run that has not started yet."""
        self.client = client
        self.run_id = run_id
        self.thread_id = thread_id
        self.token_jti: str | None = None
        self.outbox = outbox
        self.cancelled = asyncio.Event()
        self.finished = False
        self.waiting = False
        self._settings = settings
        self._on_waiting = on_waiting
        self._patch: RunPatcher | None = None

    def started(self, run: Run, token_jti: str) -> None:
        """Note that ``startRun`` or ``resumeRun`` answered ``200`` with ``run``.

        Args:
            run: The answered run (its ``version`` is the patch base).
            token_jti: The ``jti`` the run started or resumed with.
        """
        self.token_jti = token_jti
        self.waiting = False
        if self._patch is None:
            self._patch = RunPatcher(
                self.client,
                self.run_id,
                run.version,
                settings=self._settings,
                on_ended=self.cancelled.set,
            )
        else:
            self._patch.base_version = run.version

    @property
    def started_patcher(self) -> RunPatcher | None:
        """The patcher once the run started, else ``None``."""
        return self._patch

    @property
    def patch(self) -> RunPatcher:
        """The live-box patcher.

        Raises:
            RuntimeError: The run has not started yet (inside ``prepare``).
        """
        if self._patch is None:
            raise RuntimeError("the run has not started; patches are possible from execute on")
        return self._patch

    async def send(self, content_md: str, **kwargs: Any) -> Any:
        """Send a reply in the run's thread through the outbox, with ``run_id`` set.

        Args:
            content_md: The Markdown text.
            **kwargs: Further arguments of `bothy_sdk.outbox.Outbox.send_message`.

        Returns:
            The outbox entry.
        """
        return await self.outbox.send_message(
            self.thread_id, content_md, run_id=self.run_id, **kwargs
        )

    @property
    def version(self) -> int:
        """The run document version the patcher works on (0 before the start)."""
        return self._patch.base_version if self._patch is not None else 0

    async def load_context(self) -> RunContext:
        """Load the run context (``getRunContext``); render it with `bothy_sdk.context`.

        Returns:
            The context.
        """
        result = await self.client.call(get_run_context.asyncio_detailed, run_id=self.run_id)
        return expect(result, RunContext)

    async def await_input(self, status_line: str) -> None:
        """Wait for a person (``awaitRunInput``); the run goes to ``waiting_input``.

        Pending patches are sent first. The adapter resumes the run with the token of the next
        follow-up or interaction.

        Args:
            status_line: What the run waits for.
        """
        if self._patch is not None:
            await self._patch.flush()
        await self.client.call(
            await_run_input.asyncio_detailed,
            run_id=self.run_id,
            body=RunAwaitInput(status_line=status_line),
        )
        self.waiting = True
        if self._on_waiting is not None:
            await self._on_waiting()

    async def finish(
        self,
        state: str,
        reason: str | None = None,
        summary: str | None = None,
        metrics: RunMetrics | None = None,
        costs: Sequence[CostReport] = (),
    ) -> None:
        """End the run (``finishRun``) after sending pending patches.

        A flush that finds the run ended is dropped, and a ``finishRun`` answered with
        ``run_inactive`` or ``run_not_active`` is logged as already ended, not raised.

        Args:
            state: ``completed``, ``failed`` or ``cancelled``.
            reason: The reason code (for example ``runtime_error``).
            summary: A short summary.
            metrics: Final metrics.
            costs: Usage per model (09 4.10 "usage", ``costs`` in openapi).

        Raises:
            ProblemError: Another refusal of the server.
        """
        if self.finished:
            return
        self.finished = True
        if self._patch is not None:
            with contextlib.suppress(ProblemError):
                await self._patch.flush()
            await self._patch.aclose()
        body = RunFinish(state=RunFinishState(state))
        if reason is not None:
            body.reason = reason
        if summary is not None:
            body.summary = summary
        if metrics is not None:
            body.metrics = metrics
        if costs:
            body.costs = list(costs)
        try:
            await self.client.call(finish_run.asyncio_detailed, run_id=self.run_id, body=body)
        except ProblemError as exc:
            if not is_run_ended(exc):
                raise
            _LOG.info("run %s already ended on the server", self.run_id)
