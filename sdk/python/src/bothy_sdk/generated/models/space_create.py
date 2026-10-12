from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SpaceCreate")


@_attrs_define
class SpaceCreate:
    """
    Attributes:
        name (str): At most `space.name_max_chars` characters.
        icon (str | Unset): Glyph name from the list `space-icons` of the design tokens.
        color (str | Unset): Key of a category color of the design system, never a hex value.
        after_space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    name: str
    icon: str | Unset = UNSET
    color: str | Unset = UNSET
    after_space_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        icon = self.icon

        color = self.color

        after_space_id: str | Unset = UNSET
        if not isinstance(self.after_space_id, Unset):
            after_space_id = str(self.after_space_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
            }
        )
        if icon is not UNSET:
            field_dict["icon"] = icon
        if color is not UNSET:
            field_dict["color"] = color
        if after_space_id is not UNSET:
            field_dict["after_space_id"] = after_space_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        icon = d.pop("icon", UNSET)

        color = d.pop("color", UNSET)

        _after_space_id = d.pop("after_space_id", UNSET)
        after_space_id: UUID | Unset
        if isinstance(_after_space_id, Unset):
            after_space_id = UNSET
        else:
            after_space_id = UUID(_after_space_id)

        space_create = cls(
            name=name,
            icon=icon,
            color=color,
            after_space_id=after_space_id,
        )

        return space_create
