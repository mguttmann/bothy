from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SidebarStatePut")


@_attrs_define
class SidebarStatePut:
    """
    Attributes:
        collapsed (bool):
        sort_key (str | Unset):
        use_global_order (bool | Unset): True removes the personal order. Replaces sending null.
        channel_order (list[UUID] | Unset): New personal order of the channels and forums of the space, unique IDs;
            absent keeps the stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
    """

    collapsed: bool
    sort_key: str | Unset = UNSET
    use_global_order: bool | Unset = UNSET
    channel_order: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        collapsed = self.collapsed

        sort_key = self.sort_key

        use_global_order = self.use_global_order

        channel_order: list[str] | Unset = UNSET
        if not isinstance(self.channel_order, Unset):
            channel_order = []
            for channel_order_item_data in self.channel_order:
                channel_order_item = str(channel_order_item_data)
                channel_order.append(channel_order_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "collapsed": collapsed,
            }
        )
        if sort_key is not UNSET:
            field_dict["sort_key"] = sort_key
        if use_global_order is not UNSET:
            field_dict["use_global_order"] = use_global_order
        if channel_order is not UNSET:
            field_dict["channel_order"] = channel_order

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        collapsed = d.pop("collapsed")

        sort_key = d.pop("sort_key", UNSET)

        use_global_order = d.pop("use_global_order", UNSET)

        _channel_order = d.pop("channel_order", UNSET)
        channel_order: list[UUID] | Unset = UNSET
        if _channel_order is not UNSET:
            channel_order = []
            for channel_order_item_data in _channel_order:
                channel_order_item = UUID(channel_order_item_data)

                channel_order.append(channel_order_item)

        sidebar_state_put = cls(
            collapsed=collapsed,
            sort_key=sort_key,
            use_global_order=use_global_order,
            channel_order=channel_order,
        )

        return sidebar_state_put
