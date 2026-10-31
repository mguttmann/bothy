from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="SidebarChannelState")


@_attrs_define
class SidebarChannelState:
    """
    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_order (list[UUID]): Personal order of the open posts of the channel; empty uses the default order. At
            most `sidebar.order_max_items`.
        version (int):
        updated_by_device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    channel_id: UUID
    thread_order: list[UUID]
    version: int
    updated_by_device_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        thread_order = []
        for thread_order_item_data in self.thread_order:
            thread_order_item = str(thread_order_item_data)
            thread_order.append(thread_order_item)

        version = self.version

        updated_by_device_id: str | Unset = UNSET
        if not isinstance(self.updated_by_device_id, Unset):
            updated_by_device_id = str(self.updated_by_device_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel_id": channel_id,
                "thread_order": thread_order,
                "version": version,
            }
        )
        if updated_by_device_id is not UNSET:
            field_dict["updated_by_device_id"] = updated_by_device_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        thread_order = []
        _thread_order = d.pop("thread_order")
        for thread_order_item_data in _thread_order:
            thread_order_item = UUID(thread_order_item_data)

            thread_order.append(thread_order_item)

        version = d.pop("version")

        _updated_by_device_id = d.pop("updated_by_device_id", UNSET)
        updated_by_device_id: UUID | Unset
        if isinstance(_updated_by_device_id, Unset):
            updated_by_device_id = UNSET
        else:
            updated_by_device_id = UUID(_updated_by_device_id)

        sidebar_channel_state = cls(
            channel_id=channel_id,
            thread_order=thread_order,
            version=version,
            updated_by_device_id=updated_by_device_id,
        )

        sidebar_channel_state.additional_properties = d
        return sidebar_channel_state

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
