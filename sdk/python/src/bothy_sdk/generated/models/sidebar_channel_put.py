from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="SidebarChannelPut")


@_attrs_define
class SidebarChannelPut:
    """
    Attributes:
        thread_order (list[UUID]): New personal order of the posts of the channel, unique IDs; an empty list returns to
            the default order. At most `sidebar.order_max_items`.
    """

    thread_order: list[UUID]

    def to_dict(self) -> dict[str, Any]:
        thread_order = []
        for thread_order_item_data in self.thread_order:
            thread_order_item = str(thread_order_item_data)
            thread_order.append(thread_order_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "thread_order": thread_order,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_order = []
        _thread_order = d.pop("thread_order")
        for thread_order_item_data in _thread_order:
            thread_order_item = UUID(thread_order_item_data)

            thread_order.append(thread_order_item)

        sidebar_channel_put = cls(
            thread_order=thread_order,
        )

        return sidebar_channel_put
