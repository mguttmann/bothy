from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_wake_kind import RunWakeKind
from ..models.run_wake_reason import RunWakeReason

T = TypeVar("T", bound="AgentWakeFacts")


@_attrs_define
class AgentWakeFacts:
    """The wake-up of a wake run or a wake input; the runner writes its label, reason and result as the input (09 5.8.3).

    Attributes:
        kind (RunWakeKind): What makes a wake-up fire (09 5.8.1); `at` the server job `runs.wake_due`, `pr_checks` and
            `command` the runner after `run.wake.check`.
        reason (RunWakeReason): Why the wake-up exists (09 5.8.1); `agent` from `bothy-wake`, `recovery` after lost
            background work, `usage_limit` at the reset of the account's usage window.
        label (str):
        result_text (str):
    """

    kind: RunWakeKind
    reason: RunWakeReason
    label: str
    result_text: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        reason = self.reason.value

        label = self.label

        result_text = self.result_text

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
                "reason": reason,
                "label": label,
                "result_text": result_text,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = RunWakeKind(d.pop("kind"))

        reason = RunWakeReason(d.pop("reason"))

        label = d.pop("label")

        result_text = d.pop("result_text")

        agent_wake_facts = cls(
            kind=kind,
            reason=reason,
            label=label,
            result_text=result_text,
        )

        agent_wake_facts.additional_properties = d
        return agent_wake_facts

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
