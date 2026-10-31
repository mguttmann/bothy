from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.sidebar_channel_state import SidebarChannelState
    from ..models.sidebar_state import SidebarState


T = TypeVar("T", bound="SidebarStateList")


@_attrs_define
class SidebarStateList:
    """
    Attributes:
        items (list[SidebarState]):
        channels (list[SidebarChannelState] | Unset): The caller's personal post orders of channels.
    """

    items: list[SidebarState]
    channels: list[SidebarChannelState] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        channels: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.channels, Unset):
            channels = []
            for channels_item_data in self.channels:
                channels_item = channels_item_data.to_dict()
                channels.append(channels_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "items": items,
            }
        )
        if channels is not UNSET:
            field_dict["channels"] = channels

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.sidebar_channel_state import SidebarChannelState
        from ..models.sidebar_state import SidebarState

        d = dict(src_dict)
        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = SidebarState.from_dict(items_item_data)

            items.append(items_item)

        _channels = d.pop("channels", UNSET)
        channels: list[SidebarChannelState] | Unset = UNSET
        if _channels is not UNSET:
            channels = []
            for channels_item_data in _channels:
                channels_item = SidebarChannelState.from_dict(channels_item_data)

                channels.append(channels_item)

        sidebar_state_list = cls(
            items=items,
            channels=channels,
        )

        sidebar_state_list.additional_properties = d
        return sidebar_state_list

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
