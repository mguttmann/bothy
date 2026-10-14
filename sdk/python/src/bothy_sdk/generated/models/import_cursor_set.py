from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ImportCursorSet")


@_attrs_define
class ImportCursorSet:
    """
    Attributes:
        cursor (JsonObject): Free-form JSON object whose schema is named in the field description.
        stats (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    cursor: JsonObject
    stats: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        cursor = self.cursor.to_dict()

        stats: dict[str, Any] | Unset = UNSET
        if not isinstance(self.stats, Unset):
            stats = self.stats.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "cursor": cursor,
            }
        )
        if stats is not UNSET:
            field_dict["stats"] = stats

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        cursor = JsonObject.from_dict(d.pop("cursor"))

        _stats = d.pop("stats", UNSET)
        stats: JsonObject | Unset
        if isinstance(_stats, Unset):
            stats = UNSET
        else:
            stats = JsonObject.from_dict(_stats)

        import_cursor_set = cls(
            cursor=cursor,
            stats=stats,
        )

        return import_cursor_set
