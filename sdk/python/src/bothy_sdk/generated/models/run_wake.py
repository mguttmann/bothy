from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_wake_kind import RunWakeKind
from ..models.run_wake_reason import RunWakeReason
from ..models.run_wake_state import RunWakeState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_wake_condition import RunWakeCondition


T = TypeVar("T", bound="RunWake")


@_attrs_define
class RunWake:
    """A wake-up (03 `run_wakes`, 09 5.8.1): a server record that continues a thread later without a new message, on behalf
    of the person of its source run.

        Attributes:
            id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            source_run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            triggered_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            kind (RunWakeKind): What makes a wake-up fire (09 5.8.1); `at` the server job `runs.wake_due`, `pr_checks` and
                `command` the runner after `run.wake.check`.
            reason (RunWakeReason): Why the wake-up exists (09 5.8.1); `agent` from `bothy-wake`, `recovery` after lost
                background work, `usage_limit` at the reset of the account's usage window.
            label (str): Short text of the agent, at most `run.wake_label_max_chars` characters; may be empty.
            condition (RunWakeCondition): Schema `run_wake_condition.v1` (03 `run_wakes`); `repository`, `number` and
                `head_commit` for `pr_checks`, `command_line` for `command`, nothing for `at`.
            not_before (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            deadline (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            state (RunWakeState): State of a wake-up (09 5.8.1); every state other than `pending` is an end, and every end
                other than `fired` posts a note in the thread (09 5.8.4).
            created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            version (int):
            outage (int | Unset): The run's outage number of a `usage_limit` wake-up of `createRunWake`.
            state_reason (str | Unset): Reason code of the last state change (09 5.8.3, 5.8.6), for example
                `autonomous_chain_max`, `deadline_passed` or `wake_input_skipped`.
            result_text (str | Unset): The result of a fired check, as the runner cut it.
            fired_run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            fired_into_run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            ended_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    agent_id: UUID
    thread_id: UUID
    source_run_id: UUID
    triggered_by: UUID
    kind: RunWakeKind
    reason: RunWakeReason
    label: str
    condition: RunWakeCondition
    not_before: datetime.datetime
    deadline: datetime.datetime
    state: RunWakeState
    created_at: datetime.datetime
    version: int
    outage: int | Unset = UNSET
    state_reason: str | Unset = UNSET
    result_text: str | Unset = UNSET
    fired_run_id: UUID | Unset = UNSET
    fired_into_run_id: UUID | Unset = UNSET
    ended_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        agent_id = str(self.agent_id)

        thread_id = str(self.thread_id)

        source_run_id = str(self.source_run_id)

        triggered_by = str(self.triggered_by)

        kind = self.kind.value

        reason = self.reason.value

        label = self.label

        condition = self.condition.to_dict()

        not_before = self.not_before.isoformat()

        deadline = self.deadline.isoformat()

        state = self.state.value

        created_at = self.created_at.isoformat()

        version = self.version

        outage = self.outage

        state_reason = self.state_reason

        result_text = self.result_text

        fired_run_id: str | Unset = UNSET
        if not isinstance(self.fired_run_id, Unset):
            fired_run_id = str(self.fired_run_id)

        fired_into_run_id: str | Unset = UNSET
        if not isinstance(self.fired_into_run_id, Unset):
            fired_into_run_id = str(self.fired_into_run_id)

        ended_at: str | Unset = UNSET
        if not isinstance(self.ended_at, Unset):
            ended_at = self.ended_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "agent_id": agent_id,
                "thread_id": thread_id,
                "source_run_id": source_run_id,
                "triggered_by": triggered_by,
                "kind": kind,
                "reason": reason,
                "label": label,
                "condition": condition,
                "not_before": not_before,
                "deadline": deadline,
                "state": state,
                "created_at": created_at,
                "version": version,
            }
        )
        if outage is not UNSET:
            field_dict["outage"] = outage
        if state_reason is not UNSET:
            field_dict["state_reason"] = state_reason
        if result_text is not UNSET:
            field_dict["result_text"] = result_text
        if fired_run_id is not UNSET:
            field_dict["fired_run_id"] = fired_run_id
        if fired_into_run_id is not UNSET:
            field_dict["fired_into_run_id"] = fired_into_run_id
        if ended_at is not UNSET:
            field_dict["ended_at"] = ended_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_wake_condition import RunWakeCondition

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        agent_id = UUID(d.pop("agent_id"))

        thread_id = UUID(d.pop("thread_id"))

        source_run_id = UUID(d.pop("source_run_id"))

        triggered_by = UUID(d.pop("triggered_by"))

        kind = RunWakeKind(d.pop("kind"))

        reason = RunWakeReason(d.pop("reason"))

        label = d.pop("label")

        condition = RunWakeCondition.from_dict(d.pop("condition"))

        not_before = datetime.datetime.fromisoformat(d.pop("not_before"))

        deadline = datetime.datetime.fromisoformat(d.pop("deadline"))

        state = RunWakeState(d.pop("state"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        version = d.pop("version")

        outage = d.pop("outage", UNSET)

        state_reason = d.pop("state_reason", UNSET)

        result_text = d.pop("result_text", UNSET)

        _fired_run_id = d.pop("fired_run_id", UNSET)
        fired_run_id: UUID | Unset
        if isinstance(_fired_run_id, Unset):
            fired_run_id = UNSET
        else:
            fired_run_id = UUID(_fired_run_id)

        _fired_into_run_id = d.pop("fired_into_run_id", UNSET)
        fired_into_run_id: UUID | Unset
        if isinstance(_fired_into_run_id, Unset):
            fired_into_run_id = UNSET
        else:
            fired_into_run_id = UUID(_fired_into_run_id)

        _ended_at = d.pop("ended_at", UNSET)
        ended_at: datetime.datetime | Unset
        if isinstance(_ended_at, Unset):
            ended_at = UNSET
        else:
            ended_at = datetime.datetime.fromisoformat(_ended_at)

        run_wake = cls(
            id=id,
            agent_id=agent_id,
            thread_id=thread_id,
            source_run_id=source_run_id,
            triggered_by=triggered_by,
            kind=kind,
            reason=reason,
            label=label,
            condition=condition,
            not_before=not_before,
            deadline=deadline,
            state=state,
            created_at=created_at,
            version=version,
            outage=outage,
            state_reason=state_reason,
            result_text=result_text,
            fired_run_id=fired_run_id,
            fired_into_run_id=fired_into_run_id,
            ended_at=ended_at,
        )

        run_wake.additional_properties = d
        return run_wake

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
