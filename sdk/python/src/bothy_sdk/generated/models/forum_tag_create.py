from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ForumTagCreate")


@_attrs_define
class ForumTagCreate:
    """
    Attributes:
        name (str):
        color (str | Unset): Key of a category color of the design system, never a hex value.
        restricted (bool | Unset):
        after_tag_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    name: str
    color: str | Unset = UNSET
    restricted: bool | Unset = UNSET
    after_tag_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        color = self.color

        restricted = self.restricted

        after_tag_id: str | Unset = UNSET
        if not isinstance(self.after_tag_id, Unset):
            after_tag_id = str(self.after_tag_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
            }
        )
        if color is not UNSET:
            field_dict["color"] = color
        if restricted is not UNSET:
            field_dict["restricted"] = restricted
        if after_tag_id is not UNSET:
            field_dict["after_tag_id"] = after_tag_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        color = d.pop("color", UNSET)

        restricted = d.pop("restricted", UNSET)

        _after_tag_id = d.pop("after_tag_id", UNSET)
        after_tag_id: UUID | Unset
        if isinstance(_after_tag_id, Unset):
            after_tag_id = UNSET
        else:
            after_tag_id = UUID(_after_tag_id)

        forum_tag_create = cls(
            name=name,
            color=color,
            restricted=restricted,
            after_tag_id=after_tag_id,
        )

        return forum_tag_create
