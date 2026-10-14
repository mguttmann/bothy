from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="StepUpResult")


@_attrs_define
class StepUpResult:
    """
    Attributes:
        step_up_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        valid_until (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    step_up_at: datetime.datetime
    valid_until: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        step_up_at = self.step_up_at.isoformat()

        valid_until = self.valid_until.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "step_up_at": step_up_at,
                "valid_until": valid_until,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        step_up_at = datetime.datetime.fromisoformat(d.pop("step_up_at"))

        valid_until = datetime.datetime.fromisoformat(d.pop("valid_until"))

        step_up_result = cls(
            step_up_at=step_up_at,
            valid_until=valid_until,
        )

        step_up_result.additional_properties = d
        return step_up_result

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
