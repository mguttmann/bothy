from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.run_wake_create_reason import RunWakeCreateReason
from ..models.run_wake_kind import RunWakeKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_wake_condition import RunWakeCondition


T = TypeVar("T", bound="RunWakeCreate")


@_attrs_define
class RunWakeCreate:
    """
    Attributes:
        kind (RunWakeKind): What makes a wake-up fire (09 5.8.1); `at` the server job `runs.wake_due`, `pr_checks` and
            `command` the runner after `run.wake.check`.
        reason (RunWakeCreateReason):
        label (str | Unset): At most `run.wake_label_max_chars` characters, no control or bidi characters.
        condition (RunWakeCondition | Unset): Schema `run_wake_condition.v1` (03 `run_wakes`); `repository`, `number`
            and `head_commit` for `pr_checks`, `command_line` for `command`, nothing for `at`.
        not_before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        deadline (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        outage (int | Unset): The run's outage number; required exactly for `usage_limit` (18.19.10).
        deferred (bool | Unset): Set by the SDK outbox for a request it sends later; a refusal is then stored in state
            `refused` with the note and the notification (18.19.8).
    """

    kind: RunWakeKind
    reason: RunWakeCreateReason
    label: str | Unset = UNSET
    condition: RunWakeCondition | Unset = UNSET
    not_before: datetime.datetime | Unset = UNSET
    deadline: datetime.datetime | Unset = UNSET
    outage: int | Unset = UNSET
    deferred: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        reason = self.reason.value

        label = self.label

        condition: dict[str, Any] | Unset = UNSET
        if not isinstance(self.condition, Unset):
            condition = self.condition.to_dict()

        not_before: str | Unset = UNSET
        if not isinstance(self.not_before, Unset):
            not_before = self.not_before.isoformat()

        deadline: str | Unset = UNSET
        if not isinstance(self.deadline, Unset):
            deadline = self.deadline.isoformat()

        outage = self.outage

        deferred = self.deferred

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "reason": reason,
            }
        )
        if label is not UNSET:
            field_dict["label"] = label
        if condition is not UNSET:
            field_dict["condition"] = condition
        if not_before is not UNSET:
            field_dict["not_before"] = not_before
        if deadline is not UNSET:
            field_dict["deadline"] = deadline
        if outage is not UNSET:
            field_dict["outage"] = outage
        if deferred is not UNSET:
            field_dict["deferred"] = deferred

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_wake_condition import RunWakeCondition

        d = dict(src_dict)
        kind = RunWakeKind(d.pop("kind"))

        reason = RunWakeCreateReason(d.pop("reason"))

        label = d.pop("label", UNSET)

        _condition = d.pop("condition", UNSET)
        condition: RunWakeCondition | Unset
        if isinstance(_condition, Unset):
            condition = UNSET
        else:
            condition = RunWakeCondition.from_dict(_condition)

        _not_before = d.pop("not_before", UNSET)
        not_before: datetime.datetime | Unset
        if isinstance(_not_before, Unset):
            not_before = UNSET
        else:
            not_before = datetime.datetime.fromisoformat(_not_before)

        _deadline = d.pop("deadline", UNSET)
        deadline: datetime.datetime | Unset
        if isinstance(_deadline, Unset):
            deadline = UNSET
        else:
            deadline = datetime.datetime.fromisoformat(_deadline)

        outage = d.pop("outage", UNSET)

        deferred = d.pop("deferred", UNSET)

        run_wake_create = cls(
            kind=kind,
            reason=reason,
            label=label,
            condition=condition,
            not_before=not_before,
            deadline=deadline,
            outage=outage,
            deferred=deferred,
        )

        return run_wake_create
