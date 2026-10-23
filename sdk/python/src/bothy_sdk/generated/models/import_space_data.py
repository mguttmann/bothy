from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportSpaceData")


@_attrs_define
class ImportSpaceData:
    """`data` of a `space` item (origin key `space:<category_id>`). Exactly one of `name` (a new space as the mapping says
    `create`) and `existing_space_id` (equal to the mapping's `existing`).

        Attributes:
            sort_key (str): Fractional index key of the space order, derived by the tool from the Discord `position`.
            name (str | Unset):
            existing_space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    sort_key: str
    name: str | Unset = UNSET
    existing_space_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        sort_key = self.sort_key

        name = self.name

        existing_space_id: str | Unset = UNSET
        if not isinstance(self.existing_space_id, Unset):
            existing_space_id = str(self.existing_space_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "sort_key": sort_key,
            }
        )
        if name is not UNSET:
            field_dict["name"] = name
        if existing_space_id is not UNSET:
            field_dict["existing_space_id"] = existing_space_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        sort_key = d.pop("sort_key")

        name = d.pop("name", UNSET)

        _existing_space_id = d.pop("existing_space_id", UNSET)
        existing_space_id: UUID | Unset
        if isinstance(_existing_space_id, Unset):
            existing_space_id = UNSET
        else:
            existing_space_id = UUID(_existing_space_id)

        import_space_data = cls(
            sort_key=sort_key,
            name=name,
            existing_space_id=existing_space_id,
        )

        return import_space_data
