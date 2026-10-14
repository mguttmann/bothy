from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.upload_part_result import UploadPartResult


T = TypeVar("T", bound="UploadComplete")


@_attrs_define
class UploadComplete:
    """
    Attributes:
        parts (list[UploadPartResult]):
    """

    parts: list[UploadPartResult]

    def to_dict(self) -> dict[str, Any]:
        parts = []
        for parts_item_data in self.parts:
            parts_item = parts_item_data.to_dict()
            parts.append(parts_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "parts": parts,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.upload_part_result import UploadPartResult

        d = dict(src_dict)
        parts = []
        _parts = d.pop("parts")
        for parts_item_data in _parts:
            parts_item = UploadPartResult.from_dict(parts_item_data)

            parts.append(parts_item)

        upload_complete = cls(
            parts=parts,
        )

        return upload_complete
