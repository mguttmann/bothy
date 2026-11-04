from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.billing_mode import BillingMode
from ..models.run_state import RunState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.money import Money
    from ..models.run_metrics import RunMetrics
    from ..models.stats_run_chat import StatsRunChat


T = TypeVar("T", bound="RunHistoryEntry")


@_attrs_define
class RunHistoryEntry:
    """A finished run in the run history (docs/spec/25-visualization.md, section 6.7). The step tree, the last text excerpt
    and the triggering message come from `getRun`.

        Attributes:
            id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            state (RunState):
            billing_mode_expected (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly
                one person.
            cost (Money): Amount in micro units of the currency. Never a float.
            metrics (RunMetrics): Schema `run_metrics.v1`.
            step_count (int):
            queued_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            finished_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            state_reason (str | Unset):
            billing_mode_used (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
                exactly one person.
            model (str | Unset):
            summary (str | Unset):
            started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            duration_ms (int | Unset):
            chat (StatsRunChat | Unset): The chat a run belongs to, as the sidebar names it: the name of its channel (the
                project) and the title of its thread. Present only when the caller may read the thread (`thread.read`, the same
                rule as the sidebar); absent otherwise, and then a client shows neither a name nor a link
                (docs/spec/25-visualization.md, section 4, rule 6).
    """

    id: UUID
    thread_id: UUID
    channel_id: UUID
    agent_id: UUID
    state: RunState
    billing_mode_expected: BillingMode
    cost: Money
    metrics: RunMetrics
    step_count: int
    queued_at: datetime.datetime
    finished_at: datetime.datetime
    space_id: UUID | Unset = UNSET
    state_reason: str | Unset = UNSET
    billing_mode_used: BillingMode | Unset = UNSET
    model: str | Unset = UNSET
    summary: str | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    duration_ms: int | Unset = UNSET
    chat: StatsRunChat | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        agent_id = str(self.agent_id)

        state = self.state.value

        billing_mode_expected = self.billing_mode_expected.value

        cost = self.cost.to_dict()

        metrics = self.metrics.to_dict()

        step_count = self.step_count

        queued_at = self.queued_at.isoformat()

        finished_at = self.finished_at.isoformat()

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        state_reason = self.state_reason

        billing_mode_used: str | Unset = UNSET
        if not isinstance(self.billing_mode_used, Unset):
            billing_mode_used = self.billing_mode_used.value

        model = self.model

        summary = self.summary

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        duration_ms = self.duration_ms

        chat: dict[str, Any] | Unset = UNSET
        if not isinstance(self.chat, Unset):
            chat = self.chat.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "id": id,
                "thread_id": thread_id,
                "channel_id": channel_id,
                "agent_id": agent_id,
                "state": state,
                "billing_mode_expected": billing_mode_expected,
                "cost": cost,
                "metrics": metrics,
                "step_count": step_count,
                "queued_at": queued_at,
                "finished_at": finished_at,
            }
        )
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if state_reason is not UNSET:
            field_dict["state_reason"] = state_reason
        if billing_mode_used is not UNSET:
            field_dict["billing_mode_used"] = billing_mode_used
        if model is not UNSET:
            field_dict["model"] = model
        if summary is not UNSET:
            field_dict["summary"] = summary
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if duration_ms is not UNSET:
            field_dict["duration_ms"] = duration_ms
        if chat is not UNSET:
            field_dict["chat"] = chat

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money
        from ..models.run_metrics import RunMetrics
        from ..models.stats_run_chat import StatsRunChat

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        agent_id = UUID(d.pop("agent_id"))

        state = RunState(d.pop("state"))

        billing_mode_expected = BillingMode(d.pop("billing_mode_expected"))

        cost = Money.from_dict(d.pop("cost"))

        metrics = RunMetrics.from_dict(d.pop("metrics"))

        step_count = d.pop("step_count")

        queued_at = datetime.datetime.fromisoformat(d.pop("queued_at"))

        finished_at = datetime.datetime.fromisoformat(d.pop("finished_at"))

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        state_reason = d.pop("state_reason", UNSET)

        _billing_mode_used = d.pop("billing_mode_used", UNSET)
        billing_mode_used: BillingMode | Unset
        if isinstance(_billing_mode_used, Unset):
            billing_mode_used = UNSET
        else:
            billing_mode_used = BillingMode(_billing_mode_used)

        model = d.pop("model", UNSET)

        summary = d.pop("summary", UNSET)

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        duration_ms = d.pop("duration_ms", UNSET)

        _chat = d.pop("chat", UNSET)
        chat: StatsRunChat | Unset
        if isinstance(_chat, Unset):
            chat = UNSET
        else:
            chat = StatsRunChat.from_dict(_chat)

        run_history_entry = cls(
            id=id,
            thread_id=thread_id,
            channel_id=channel_id,
            agent_id=agent_id,
            state=state,
            billing_mode_expected=billing_mode_expected,
            cost=cost,
            metrics=metrics,
            step_count=step_count,
            queued_at=queued_at,
            finished_at=finished_at,
            space_id=space_id,
            state_reason=state_reason,
            billing_mode_used=billing_mode_used,
            model=model,
            summary=summary,
            started_at=started_at,
            duration_ms=duration_ms,
            chat=chat,
        )

        return run_history_entry
