from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="CommandOptionValue")


@_attrs_define
class CommandOptionValue:
    """
    Attributes:
        name (str):
        value (str):
    """

    name: str
    value: str

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        value = self.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "value": value,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        value = d.pop("value")

        command_option_value = cls(
            name=name,
            value=value,
        )

        return command_option_value
