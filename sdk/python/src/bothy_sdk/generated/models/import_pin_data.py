from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ImportPinData")


@_attrs_define
class ImportPinData:
    """Data of a `pin` item (22, 6.6); `pinned_by` is the importing owner.

    Attributes:
        message_origin_key (str):
    """

    message_origin_key: str

    def to_dict(self) -> dict[str, Any]:
        message_origin_key = self.message_origin_key

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "message_origin_key": message_origin_key,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_origin_key = d.pop("message_origin_key")

        import_pin_data = cls(
            message_origin_key=message_origin_key,
        )

        return import_pin_data
