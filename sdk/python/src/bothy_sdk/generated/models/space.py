from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="Space")


@_attrs_define
class Space:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        sort_key (str): Fractional index, compared bytewise.
        version (int):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        icon (str | Unset): Glyph name from the list `space-icons` of the design tokens (`kernel/designkeys`). No emoji.
        color (str | Unset): Key of a category color of the design system, never a hex value.
        default_operator_agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        archived_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    name: str
    sort_key: str
    version: int
    created_at: datetime.datetime
    icon: str | Unset = UNSET
    color: str | Unset = UNSET
    default_operator_agent_id: UUID | Unset = UNSET
    archived_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        sort_key = self.sort_key

        version = self.version

        created_at = self.created_at.isoformat()

        icon = self.icon

        color = self.color

        default_operator_agent_id: str | Unset = UNSET
        if not isinstance(self.default_operator_agent_id, Unset):
            default_operator_agent_id = str(self.default_operator_agent_id)

        archived_at: str | Unset = UNSET
        if not isinstance(self.archived_at, Unset):
            archived_at = self.archived_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "sort_key": sort_key,
                "version": version,
                "created_at": created_at,
            }
        )
        if icon is not UNSET:
            field_dict["icon"] = icon
        if color is not UNSET:
            field_dict["color"] = color
        if default_operator_agent_id is not UNSET:
            field_dict["default_operator_agent_id"] = default_operator_agent_id
        if archived_at is not UNSET:
            field_dict["archived_at"] = archived_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        sort_key = d.pop("sort_key")

        version = d.pop("version")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        icon = d.pop("icon", UNSET)

        color = d.pop("color", UNSET)

        _default_operator_agent_id = d.pop("default_operator_agent_id", UNSET)
        default_operator_agent_id: UUID | Unset
        if isinstance(_default_operator_agent_id, Unset):
            default_operator_agent_id = UNSET
        else:
            default_operator_agent_id = UUID(_default_operator_agent_id)

        _archived_at = d.pop("archived_at", UNSET)
        archived_at: datetime.datetime | Unset
        if isinstance(_archived_at, Unset):
            archived_at = UNSET
        else:
            archived_at = datetime.datetime.fromisoformat(_archived_at)

        space = cls(
            id=id,
            name=name,
            sort_key=sort_key,
            version=version,
            created_at=created_at,
            icon=icon,
            color=color,
            default_operator_agent_id=default_operator_agent_id,
            archived_at=archived_at,
        )

        space.additional_properties = d
        return space

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
