from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="ForumTag")


@_attrs_define
class ForumTag:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        sort_key (str):
        restricted (bool):
        color (str | Unset): Key of a category color of the design system, never a hex value.
    """

    id: UUID
    channel_id: UUID
    name: str
    sort_key: str
    restricted: bool
    color: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        channel_id = str(self.channel_id)

        name = self.name

        sort_key = self.sort_key

        restricted = self.restricted

        color = self.color

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "channel_id": channel_id,
                "name": name,
                "sort_key": sort_key,
                "restricted": restricted,
            }
        )
        if color is not UNSET:
            field_dict["color"] = color

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        channel_id = UUID(d.pop("channel_id"))

        name = d.pop("name")

        sort_key = d.pop("sort_key")

        restricted = d.pop("restricted")

        color = d.pop("color", UNSET)

        forum_tag = cls(
            id=id,
            channel_id=channel_id,
            name=name,
            sort_key=sort_key,
            restricted=restricted,
            color=color,
        )

        forum_tag.additional_properties = d
        return forum_tag

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
