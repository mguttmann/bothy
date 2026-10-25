from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..models.run_state import RunState
from ..models.run_trigger_kind import RunTriggerKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.money import Money
    from ..models.run_followups import RunFollowups
    from ..models.run_metrics import RunMetrics
    from ..models.run_plan import RunPlan
    from ..models.run_steps import RunSteps


T = TypeVar("T", bound="Run")


@_attrs_define
class Run:
    """Run document (`run_document.v1`), the content of the live box.

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
        metrics (RunMetrics): Schema `run_metrics.v1`.
        cost (Money): Amount in micro units of the currency. Never a float.
        steps (RunSteps): Steps keyed by step ID; order comes from `ordinal`.
        followups (RunFollowups): Follow-ups keyed by message ID.
        queued_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        agent_session_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        trigger_kind (RunTriggerKind | Unset):
        triggering_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        triggering_command_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        triggering_interaction_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state_reason (str | Unset):
        billing_mode_used (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        model (str | Unset):
        status_line (str | Unset):
        last_text_excerpt (str | Unset):
        summary (str | Unset):
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        duration_ms (int | Unset):
        plan (RunPlan | Unset): Schema `run_plan.v1`, shown in the live box.
        stall_warned_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
    """

    id: UUID
    thread_id: UUID
    channel_id: UUID
    agent_id: UUID
    triggered_by: UUID
    state: RunState
    version: int
    billing_mode_expected: BillingMode
    metrics: RunMetrics
    cost: Money
    steps: RunSteps
    followups: RunFollowups
    queued_at: datetime.datetime
    agent_session_id: UUID | Unset = UNSET
    trigger_kind: RunTriggerKind | Unset = UNSET
    triggering_message_id: UUID | Unset = UNSET
    triggering_command_id: UUID | Unset = UNSET
    triggering_interaction_id: UUID | Unset = UNSET
    state_reason: str | Unset = UNSET
    billing_mode_used: BillingMode | Unset = UNSET
    model: str | Unset = UNSET
    status_line: str | Unset = UNSET
    last_text_excerpt: str | Unset = UNSET
    summary: str | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    duration_ms: int | Unset = UNSET
    plan: RunPlan | Unset = UNSET
    stall_warned_at: datetime.datetime | Unset = UNSET
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

        metrics = self.metrics.to_dict()

        cost = self.cost.to_dict()

        steps = self.steps.to_dict()

        followups = self.followups.to_dict()

        queued_at = self.queued_at.isoformat()

        agent_session_id: str | Unset = UNSET
        if not isinstance(self.agent_session_id, Unset):
            agent_session_id = str(self.agent_session_id)

        trigger_kind: str | Unset = UNSET
        if not isinstance(self.trigger_kind, Unset):
            trigger_kind = self.trigger_kind.value

        triggering_message_id: str | Unset = UNSET
        if not isinstance(self.triggering_message_id, Unset):
            triggering_message_id = str(self.triggering_message_id)

        triggering_command_id: str | Unset = UNSET
        if not isinstance(self.triggering_command_id, Unset):
            triggering_command_id = str(self.triggering_command_id)

        triggering_interaction_id: str | Unset = UNSET
        if not isinstance(self.triggering_interaction_id, Unset):
            triggering_interaction_id = str(self.triggering_interaction_id)

        state_reason = self.state_reason

        billing_mode_used: str | Unset = UNSET
        if not isinstance(self.billing_mode_used, Unset):
            billing_mode_used = self.billing_mode_used.value

        model = self.model

        status_line = self.status_line

        last_text_excerpt = self.last_text_excerpt

        summary = self.summary

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        duration_ms = self.duration_ms

        plan: dict[str, Any] | Unset = UNSET
        if not isinstance(self.plan, Unset):
            plan = self.plan.to_dict()

        stall_warned_at: str | Unset = UNSET
        if not isinstance(self.stall_warned_at, Unset):
            stall_warned_at = self.stall_warned_at.isoformat()

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
                "metrics": metrics,
                "cost": cost,
                "steps": steps,
                "followups": followups,
                "queued_at": queued_at,
            }
        )
        if agent_session_id is not UNSET:
            field_dict["agent_session_id"] = agent_session_id
        if trigger_kind is not UNSET:
            field_dict["trigger_kind"] = trigger_kind
        if triggering_message_id is not UNSET:
            field_dict["triggering_message_id"] = triggering_message_id
        if triggering_command_id is not UNSET:
            field_dict["triggering_command_id"] = triggering_command_id
        if triggering_interaction_id is not UNSET:
            field_dict["triggering_interaction_id"] = triggering_interaction_id
        if state_reason is not UNSET:
            field_dict["state_reason"] = state_reason
        if billing_mode_used is not UNSET:
            field_dict["billing_mode_used"] = billing_mode_used
        if model is not UNSET:
            field_dict["model"] = model
        if status_line is not UNSET:
            field_dict["status_line"] = status_line
        if last_text_excerpt is not UNSET:
            field_dict["last_text_excerpt"] = last_text_excerpt
        if summary is not UNSET:
            field_dict["summary"] = summary
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at
        if duration_ms is not UNSET:
            field_dict["duration_ms"] = duration_ms
        if plan is not UNSET:
            field_dict["plan"] = plan
        if stall_warned_at is not UNSET:
            field_dict["stall_warned_at"] = stall_warned_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money
        from ..models.run_followups import RunFollowups
        from ..models.run_metrics import RunMetrics
        from ..models.run_plan import RunPlan
        from ..models.run_steps import RunSteps

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        agent_id = UUID(d.pop("agent_id"))

        triggered_by = UUID(d.pop("triggered_by"))

        state = RunState(d.pop("state"))

        version = d.pop("version")

        billing_mode_expected = BillingMode(d.pop("billing_mode_expected"))

        metrics = RunMetrics.from_dict(d.pop("metrics"))

        cost = Money.from_dict(d.pop("cost"))

        steps = RunSteps.from_dict(d.pop("steps"))

        followups = RunFollowups.from_dict(d.pop("followups"))

        queued_at = datetime.datetime.fromisoformat(d.pop("queued_at"))

        _agent_session_id = d.pop("agent_session_id", UNSET)
        agent_session_id: UUID | Unset
        if isinstance(_agent_session_id, Unset):
            agent_session_id = UNSET
        else:
            agent_session_id = UUID(_agent_session_id)

        _trigger_kind = d.pop("trigger_kind", UNSET)
        trigger_kind: RunTriggerKind | Unset
        if isinstance(_trigger_kind, Unset):
            trigger_kind = UNSET
        else:
            trigger_kind = RunTriggerKind(_trigger_kind)

        _triggering_message_id = d.pop("triggering_message_id", UNSET)
        triggering_message_id: UUID | Unset
        if isinstance(_triggering_message_id, Unset):
            triggering_message_id = UNSET
        else:
            triggering_message_id = UUID(_triggering_message_id)

        _triggering_command_id = d.pop("triggering_command_id", UNSET)
        triggering_command_id: UUID | Unset
        if isinstance(_triggering_command_id, Unset):
            triggering_command_id = UNSET
        else:
            triggering_command_id = UUID(_triggering_command_id)

        _triggering_interaction_id = d.pop("triggering_interaction_id", UNSET)
        triggering_interaction_id: UUID | Unset
        if isinstance(_triggering_interaction_id, Unset):
            triggering_interaction_id = UNSET
        else:
            triggering_interaction_id = UUID(_triggering_interaction_id)

        state_reason = d.pop("state_reason", UNSET)

        _billing_mode_used = d.pop("billing_mode_used", UNSET)
        billing_mode_used: BillingMode | Unset
        if isinstance(_billing_mode_used, Unset):
            billing_mode_used = UNSET
        else:
            billing_mode_used = BillingMode(_billing_mode_used)

        model = d.pop("model", UNSET)

        status_line = d.pop("status_line", UNSET)

        last_text_excerpt = d.pop("last_text_excerpt", UNSET)

        summary = d.pop("summary", UNSET)

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

        duration_ms = d.pop("duration_ms", UNSET)

        _plan = d.pop("plan", UNSET)
        plan: RunPlan | Unset
        if isinstance(_plan, Unset):
            plan = UNSET
        else:
            plan = RunPlan.from_dict(_plan)

        _stall_warned_at = d.pop("stall_warned_at", UNSET)
        stall_warned_at: datetime.datetime | Unset
        if isinstance(_stall_warned_at, Unset):
            stall_warned_at = UNSET
        else:
            stall_warned_at = datetime.datetime.fromisoformat(_stall_warned_at)

        run = cls(
            id=id,
            thread_id=thread_id,
            channel_id=channel_id,
            agent_id=agent_id,
            triggered_by=triggered_by,
            state=state,
            version=version,
            billing_mode_expected=billing_mode_expected,
            metrics=metrics,
            cost=cost,
            steps=steps,
            followups=followups,
            queued_at=queued_at,
            agent_session_id=agent_session_id,
            trigger_kind=trigger_kind,
            triggering_message_id=triggering_message_id,
            triggering_command_id=triggering_command_id,
            triggering_interaction_id=triggering_interaction_id,
            state_reason=state_reason,
            billing_mode_used=billing_mode_used,
            model=model,
            status_line=status_line,
            last_text_excerpt=last_text_excerpt,
            summary=summary,
            started_at=started_at,
            finished_at=finished_at,
            duration_ms=duration_ms,
            plan=plan,
            stall_warned_at=stall_warned_at,
        )

        run.additional_properties = d
        return run

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
