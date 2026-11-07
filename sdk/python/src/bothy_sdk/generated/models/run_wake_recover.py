from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.run_wake_recover_reason import RunWakeRecoverReason
from ..types import UNSET, Unset

T = TypeVar("T", bound="RunWakeRecover")


@_attrs_define
class RunWakeRecover:
    """
    Attributes:
        reason (RunWakeRecoverReason):
        label (str | Unset): At most `run.wake_label_max_chars` characters, no control or bidi characters.
        not_before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    reason: RunWakeRecoverReason
    label: str | Unset = UNSET
    not_before: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        reason = self.reason.value

        label = self.label

        not_before: str | Unset = UNSET
        if not isinstance(self.not_before, Unset):
            not_before = self.not_before.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "reason": reason,
            }
        )
        if label is not UNSET:
            field_dict["label"] = label
        if not_before is not UNSET:
            field_dict["not_before"] = not_before

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        reason = RunWakeRecoverReason(d.pop("reason"))

        label = d.pop("label", UNSET)

        _not_before = d.pop("not_before", UNSET)
        not_before: datetime.datetime | Unset
        if isinstance(_not_before, Unset):
            not_before = UNSET
        else:
            not_before = datetime.datetime.fromisoformat(_not_before)

        run_wake_recover = cls(
            reason=reason,
            label=label,
            not_before=not_before,
        )

        return run_wake_recover
