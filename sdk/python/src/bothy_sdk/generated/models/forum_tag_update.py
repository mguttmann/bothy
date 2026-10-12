from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ForumTagUpdate")


@_attrs_define
class ForumTagUpdate:
    """
    Attributes:
        name (str | Unset):
        color (str | Unset): Key of a category color of the design system, never a hex value.
        clear_color (bool | Unset): Removes the color; not together with `color`. There is no JSON null (R-API-03).
        restricted (bool | Unset):
    """

    name: str | Unset = UNSET
    color: str | Unset = UNSET
    clear_color: bool | Unset = UNSET
    restricted: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        color = self.color

        clear_color = self.clear_color

        restricted = self.restricted

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if color is not UNSET:
            field_dict["color"] = color
        if clear_color is not UNSET:
            field_dict["clear_color"] = clear_color
        if restricted is not UNSET:
            field_dict["restricted"] = restricted

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        color = d.pop("color", UNSET)

        clear_color = d.pop("clear_color", UNSET)

        restricted = d.pop("restricted", UNSET)

        forum_tag_update = cls(
            name=name,
            color=color,
            clear_color=clear_color,
            restricted=restricted,
        )

        return forum_tag_update
