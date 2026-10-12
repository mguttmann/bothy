from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="SidebarState")


@_attrs_define
class SidebarState:
    """
    Attributes:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        collapsed (bool):
        version (int):
        sort_key (str | Unset): Personal order; absent uses the global order.
        updated_by_device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    space_id: UUID
    collapsed: bool
    version: int
    sort_key: str | Unset = UNSET
    updated_by_device_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        space_id = str(self.space_id)

        collapsed = self.collapsed

        version = self.version

        sort_key = self.sort_key

        updated_by_device_id: str | Unset = UNSET
        if not isinstance(self.updated_by_device_id, Unset):
            updated_by_device_id = str(self.updated_by_device_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "space_id": space_id,
                "collapsed": collapsed,
                "version": version,
            }
        )
        if sort_key is not UNSET:
            field_dict["sort_key"] = sort_key
        if updated_by_device_id is not UNSET:
            field_dict["updated_by_device_id"] = updated_by_device_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        space_id = UUID(d.pop("space_id"))

        collapsed = d.pop("collapsed")

        version = d.pop("version")

        sort_key = d.pop("sort_key", UNSET)

        _updated_by_device_id = d.pop("updated_by_device_id", UNSET)
        updated_by_device_id: UUID | Unset
        if isinstance(_updated_by_device_id, Unset):
            updated_by_device_id = UNSET
        else:
            updated_by_device_id = UUID(_updated_by_device_id)

        sidebar_state = cls(
            space_id=space_id,
            collapsed=collapsed,
            version=version,
            sort_key=sort_key,
            updated_by_device_id=updated_by_device_id,
        )

        sidebar_state.additional_properties = d
        return sidebar_state

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
