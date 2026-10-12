from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.gdpr_decision_decision import GdprDecisionDecision

T = TypeVar("T", bound="GdprDecision")


@_attrs_define
class GdprDecision:
    """
    Attributes:
        decision (GdprDecisionDecision):
        reason (str):
    """

    decision: GdprDecisionDecision
    reason: str

    def to_dict(self) -> dict[str, Any]:
        decision = self.decision.value

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "decision": decision,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        decision = GdprDecisionDecision(d.pop("decision"))

        reason = d.pop("reason")

        gdpr_decision = cls(
            decision=decision,
            reason=reason,
        )

        return gdpr_decision
