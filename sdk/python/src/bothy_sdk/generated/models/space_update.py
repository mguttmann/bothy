from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SpaceUpdate")


@_attrs_define
class SpaceUpdate:
    """Changes only the fields sent; `icon` and `color` cannot be cleared.

    Attributes:
        name (str | Unset): At most `space.name_max_chars` characters.
        icon (str | Unset): Glyph name from the list `space-icons` of the design tokens.
        color (str | Unset): Key of a category colour of the design system, never a hex value.
    """

    name: str | Unset = UNSET
    icon: str | Unset = UNSET
    color: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        icon = self.icon

        color = self.color

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if icon is not UNSET:
            field_dict["icon"] = icon
        if color is not UNSET:
            field_dict["color"] = color

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        icon = d.pop("icon", UNSET)

        color = d.pop("color", UNSET)

        space_update = cls(
            name=name,
            icon=icon,
            color=color,
        )

        return space_update
