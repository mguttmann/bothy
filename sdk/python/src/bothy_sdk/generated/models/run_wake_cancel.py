from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="RunWakeCancel")


@_attrs_define
class RunWakeCancel:
    """
    Attributes:
        reason (str | Unset): Reason code of a cancel by the agent, for example `usage_cleared`; a person's cancel has
            `cancelled_by_person`.
    """

    reason: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        reason = d.pop("reason", UNSET)

        run_wake_cancel = cls(
            reason=reason,
        )

        return run_wake_cancel
