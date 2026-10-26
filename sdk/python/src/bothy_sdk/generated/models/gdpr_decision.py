from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.gdpr_decision_decision import GdprDecisionDecision
from ..models.gdpr_decision_extend_months import GdprDecisionExtendMonths
from ..types import UNSET, Unset

T = TypeVar("T", bound="GdprDecision")


@_attrs_define
class GdprDecision:
    """
    Attributes:
        decision (GdprDecisionDecision):
        reason (str): Communicated to the person and recorded in the audit entry of the decision; with `extend_months`
            the justification of the extension.
        extend_months (GdprDecisionExtendMonths | Unset): Extends `due_at` once by one or two calendar months (Art.
            12(3) GDPR). Accepted only before the original `due_at`; a second extension or one after the deadline is `409
            conflict` with reason `extension_not_allowed`.
    """

    decision: GdprDecisionDecision
    reason: str
    extend_months: GdprDecisionExtendMonths | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        decision = self.decision.value

        reason = self.reason

        extend_months: int | Unset = UNSET
        if not isinstance(self.extend_months, Unset):
            extend_months = self.extend_months.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "decision": decision,
                "reason": reason,
            }
        )
        if extend_months is not UNSET:
            field_dict["extend_months"] = extend_months

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        decision = GdprDecisionDecision(d.pop("decision"))

        reason = d.pop("reason")

        _extend_months = d.pop("extend_months", UNSET)
        extend_months: GdprDecisionExtendMonths | Unset
        if isinstance(_extend_months, Unset):
            extend_months = UNSET
        else:
            extend_months = GdprDecisionExtendMonths(_extend_months)

        gdpr_decision = cls(
            decision=decision,
            reason=reason,
            extend_months=extend_months,
        )

        return gdpr_decision
