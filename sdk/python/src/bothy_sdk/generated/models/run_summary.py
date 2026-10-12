from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..models.run_state import RunState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.money import Money


T = TypeVar("T", bound="RunSummary")


@_attrs_define
class RunSummary:
    """Summary for sidebars, "running now" and collapsed live boxes.

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        triggered_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (RunState):
        version (int):
        billing_mode_expected (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly
            one person.
        cost (Money): Amount in micro units of the currency. Never a float.
        step_count (int):
        queued_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        state_reason (str | Unset):
        billing_mode_used (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        status_line (str | Unset):
        summary (str | Unset):
        duration_ms (int | Unset):
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    thread_id: UUID
    channel_id: UUID
    agent_id: UUID
    triggered_by: UUID
    state: RunState
    version: int
    billing_mode_expected: BillingMode
    cost: Money
    step_count: int
    queued_at: datetime.datetime
    state_reason: str | Unset = UNSET
    billing_mode_used: BillingMode | Unset = UNSET
    status_line: str | Unset = UNSET
    summary: str | Unset = UNSET
    duration_ms: int | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        agent_id = str(self.agent_id)

        triggered_by = str(self.triggered_by)

        state = self.state.value

        version = self.version

        billing_mode_expected = self.billing_mode_expected.value

        cost = self.cost.to_dict()

        step_count = self.step_count

        queued_at = self.queued_at.isoformat()

        state_reason = self.state_reason

        billing_mode_used: str | Unset = UNSET
        if not isinstance(self.billing_mode_used, Unset):
            billing_mode_used = self.billing_mode_used.value

        status_line = self.status_line

        summary = self.summary

        duration_ms = self.duration_ms

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "thread_id": thread_id,
                "channel_id": channel_id,
                "agent_id": agent_id,
                "triggered_by": triggered_by,
                "state": state,
                "version": version,
                "billing_mode_expected": billing_mode_expected,
                "cost": cost,
                "step_count": step_count,
                "queued_at": queued_at,
            }
        )
        if state_reason is not UNSET:
            field_dict["state_reason"] = state_reason
        if billing_mode_used is not UNSET:
            field_dict["billing_mode_used"] = billing_mode_used
        if status_line is not UNSET:
            field_dict["status_line"] = status_line
        if summary is not UNSET:
            field_dict["summary"] = summary
        if duration_ms is not UNSET:
            field_dict["duration_ms"] = duration_ms
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        agent_id = UUID(d.pop("agent_id"))

        triggered_by = UUID(d.pop("triggered_by"))

        state = RunState(d.pop("state"))

        version = d.pop("version")

        billing_mode_expected = BillingMode(d.pop("billing_mode_expected"))

        cost = Money.from_dict(d.pop("cost"))

        step_count = d.pop("step_count")

        queued_at = datetime.datetime.fromisoformat(d.pop("queued_at"))

        state_reason = d.pop("state_reason", UNSET)

        _billing_mode_used = d.pop("billing_mode_used", UNSET)
        billing_mode_used: BillingMode | Unset
        if isinstance(_billing_mode_used, Unset):
            billing_mode_used = UNSET
        else:
            billing_mode_used = BillingMode(_billing_mode_used)

        status_line = d.pop("status_line", UNSET)

        summary = d.pop("summary", UNSET)

        duration_ms = d.pop("duration_ms", UNSET)

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _finished_at = d.pop("finished_at", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        run_summary = cls(
            id=id,
            thread_id=thread_id,
            channel_id=channel_id,
            agent_id=agent_id,
            triggered_by=triggered_by,
            state=state,
            version=version,
            billing_mode_expected=billing_mode_expected,
            cost=cost,
            step_count=step_count,
            queued_at=queued_at,
            state_reason=state_reason,
            billing_mode_used=billing_mode_used,
            status_line=status_line,
            summary=summary,
            duration_ms=duration_ms,
            started_at=started_at,
            finished_at=finished_at,
        )

        run_summary.additional_properties = d
        return run_summary

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
