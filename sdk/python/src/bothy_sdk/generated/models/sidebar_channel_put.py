from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SidebarChannelPut")


@_attrs_define
class SidebarChannelPut:
    """
    Attributes:
        thread_order (list[UUID] | Unset): New personal order of the posts of the channel, unique IDs; absent keeps the
            stored order, an empty list returns to the default order. At most `sidebar.order_max_items`.
        hidden (bool | Unset): true archives the channel in the caller's own sidebar (sets `hidden_at`, kept when it is
            already set), false brings it back (clears `hidden_at`); absent keeps the stored state (01 6.22).
    """

    thread_order: list[UUID] | Unset = UNSET
    hidden: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        thread_order: list[str] | Unset = UNSET
        if not isinstance(self.thread_order, Unset):
            thread_order = []
            for thread_order_item_data in self.thread_order:
                thread_order_item = str(thread_order_item_data)
                thread_order.append(thread_order_item)

        hidden = self.hidden

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if thread_order is not UNSET:
            field_dict["thread_order"] = thread_order
        if hidden is not UNSET:
            field_dict["hidden"] = hidden

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _thread_order = d.pop("thread_order", UNSET)
        thread_order: list[UUID] | Unset = UNSET
        if _thread_order is not UNSET:
            thread_order = []
            for thread_order_item_data in _thread_order:
                thread_order_item = UUID(thread_order_item_data)

                thread_order.append(thread_order_item)

        hidden = d.pop("hidden", UNSET)

        sidebar_channel_put = cls(
            thread_order=thread_order,
            hidden=hidden,
        )

        return sidebar_channel_put
