from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.import_kind import ImportKind

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ImportSourceCreate")


@_attrs_define
class ImportSourceCreate:
    """
    Attributes:
        kind (ImportKind):
        name (str):
        config (JsonObject): Free-form JSON object whose schema is named in the field description.
    """

    kind: ImportKind
    name: str
    config: JsonObject

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        name = self.name

        config = self.config.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "name": name,
                "config": config,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        kind = ImportKind(d.pop("kind"))

        name = d.pop("name")

        config = JsonObject.from_dict(d.pop("config"))

        import_source_create = cls(
            kind=kind,
            name=name,
            config=config,
        )

        return import_source_create
