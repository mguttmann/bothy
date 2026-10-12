from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_object_item_kind import ImportObjectItemKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ImportObject")


@_attrs_define
class ImportObject:
    """
    Attributes:
        origin_key (str):
        item_kind (ImportObjectItemKind):
        data (JsonObject): Free-form JSON object whose schema is named in the field description.
        content_sha256 (str | Unset): Hex SHA-256 of the converted content; a different value on a later run is counted
            as changed at the source.
    """

    origin_key: str
    item_kind: ImportObjectItemKind
    data: JsonObject
    content_sha256: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        origin_key = self.origin_key

        item_kind = self.item_kind.value

        data = self.data.to_dict()

        content_sha256 = self.content_sha256

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "origin_key": origin_key,
                "item_kind": item_kind,
                "data": data,
            }
        )
        if content_sha256 is not UNSET:
            field_dict["content_sha256"] = content_sha256

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        origin_key = d.pop("origin_key")

        item_kind = ImportObjectItemKind(d.pop("item_kind"))

        data = JsonObject.from_dict(d.pop("data"))

        content_sha256 = d.pop("content_sha256", UNSET)

        import_object = cls(
            origin_key=origin_key,
            item_kind=item_kind,
            data=data,
            content_sha256=content_sha256,
        )

        return import_object
