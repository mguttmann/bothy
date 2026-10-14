from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..models.stats_live_run_state import StatsLiveRunState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.stats_live_step import StatsLiveStep


T = TypeVar("T", bound="StatsLiveRun")


@_attrs_define
class StatsLiveRun:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (StatsLiveRunState):
        queued_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        approval_pending (bool): An approval card of this run waits for a decision.
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        step (StatsLiveStep | Unset): Current step of a run; only kind and name (for tools the tool name), never title
            or detail.
        cost_micros (int | Unset): Cost so far; only with `cost.read.aggregate`.
        currency (str | Unset):
        cost_is_estimate (bool | Unset):
    """

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    space_id: UUID
    agent_id: UUID
    state: StatsLiveRunState
    queued_at: datetime.datetime
    billing_mode: BillingMode
    approval_pending: bool
    started_at: datetime.datetime | Unset = UNSET
    step: StatsLiveStep | Unset = UNSET
    cost_micros: int | Unset = UNSET
    currency: str | Unset = UNSET
    cost_is_estimate: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        space_id = str(self.space_id)

        agent_id = str(self.agent_id)

        state = self.state.value

        queued_at = self.queued_at.isoformat()

        billing_mode = self.billing_mode.value

        approval_pending = self.approval_pending

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        step: dict[str, Any] | Unset = UNSET
        if not isinstance(self.step, Unset):
            step = self.step.to_dict()

        cost_micros = self.cost_micros

        currency = self.currency

        cost_is_estimate = self.cost_is_estimate

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "thread_id": thread_id,
                "channel_id": channel_id,
                "space_id": space_id,
                "agent_id": agent_id,
                "state": state,
                "queued_at": queued_at,
                "billing_mode": billing_mode,
                "approval_pending": approval_pending,
            }
        )
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if step is not UNSET:
            field_dict["step"] = step
        if cost_micros is not UNSET:
            field_dict["cost_micros"] = cost_micros
        if currency is not UNSET:
            field_dict["currency"] = currency
        if cost_is_estimate is not UNSET:
            field_dict["cost_is_estimate"] = cost_is_estimate

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_live_step import StatsLiveStep

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        space_id = UUID(d.pop("space_id"))

        agent_id = UUID(d.pop("agent_id"))

        state = StatsLiveRunState(d.pop("state"))

        queued_at = datetime.datetime.fromisoformat(d.pop("queued_at"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        approval_pending = d.pop("approval_pending")

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _step = d.pop("step", UNSET)
        step: StatsLiveStep | Unset
        if isinstance(_step, Unset):
            step = UNSET
        else:
            step = StatsLiveStep.from_dict(_step)

        cost_micros = d.pop("cost_micros", UNSET)

        currency = d.pop("currency", UNSET)

        cost_is_estimate = d.pop("cost_is_estimate", UNSET)

        stats_live_run = cls(
            run_id=run_id,
            thread_id=thread_id,
            channel_id=channel_id,
            space_id=space_id,
            agent_id=agent_id,
            state=state,
            queued_at=queued_at,
            billing_mode=billing_mode,
            approval_pending=approval_pending,
            started_at=started_at,
            step=step,
            cost_micros=cost_micros,
            currency=currency,
            cost_is_estimate=cost_is_estimate,
        )

        stats_live_run.additional_properties = d
        return stats_live_run

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
