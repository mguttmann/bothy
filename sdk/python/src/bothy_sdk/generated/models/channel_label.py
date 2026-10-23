from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ChannelLabel")


@_attrs_define
class ChannelLabel:
    """Name and space of a channel named on a page.

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
    """

    id: UUID
    space_id: UUID
    name: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        space_id = str(self.space_id)

        name = self.name

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "space_id": space_id,
                "name": name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        space_id = UUID(d.pop("space_id"))

        name = d.pop("name")

        channel_label = cls(
            id=id,
            space_id=space_id,
            name=name,
        )

        channel_label.additional_properties = d
        return channel_label

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
