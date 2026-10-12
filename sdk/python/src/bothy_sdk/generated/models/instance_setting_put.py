from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="InstanceSettingPut")


@_attrs_define
class InstanceSettingPut:
    """
    Attributes:
        value (Any): Any JSON value except null. The schema of the value is defined by its key.
    """

    value: Any

    def to_dict(self) -> dict[str, Any]:
        value = self.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "value": value,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        value = d.pop("value")

        instance_setting_put = cls(
            value=value,
        )

        return instance_setting_put
