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
        pinned (bool | Unset): true pins the channel to the top of its space in the caller's own sidebar (sets
            `pinned_at`, kept when it is already set), false unpins it (clears `pinned_at`); absent keeps the stored state
            (01 6.24).
        pinned_threads (list[UUID] | Unset): New list of the posts of the channel pinned to the top of the forum in the
            caller's own sidebar, in pin order, unique IDs; absent keeps the stored list, an empty list unpins every post
            (01 6.24). At most `sidebar.order_max_items`.
    """

    thread_order: list[UUID] | Unset = UNSET
    hidden: bool | Unset = UNSET
    pinned: bool | Unset = UNSET
    pinned_threads: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        thread_order: list[str] | Unset = UNSET
        if not isinstance(self.thread_order, Unset):
            thread_order = []
            for thread_order_item_data in self.thread_order:
                thread_order_item = str(thread_order_item_data)
                thread_order.append(thread_order_item)

        hidden = self.hidden

        pinned = self.pinned

        pinned_threads: list[str] | Unset = UNSET
        if not isinstance(self.pinned_threads, Unset):
            pinned_threads = []
            for pinned_threads_item_data in self.pinned_threads:
                pinned_threads_item = str(pinned_threads_item_data)
                pinned_threads.append(pinned_threads_item)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if thread_order is not UNSET:
            field_dict["thread_order"] = thread_order
        if hidden is not UNSET:
            field_dict["hidden"] = hidden
        if pinned is not UNSET:
            field_dict["pinned"] = pinned
        if pinned_threads is not UNSET:
            field_dict["pinned_threads"] = pinned_threads

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

        pinned = d.pop("pinned", UNSET)

        _pinned_threads = d.pop("pinned_threads", UNSET)
        pinned_threads: list[UUID] | Unset = UNSET
        if _pinned_threads is not UNSET:
            pinned_threads = []
            for pinned_threads_item_data in _pinned_threads:
                pinned_threads_item = UUID(pinned_threads_item_data)

                pinned_threads.append(pinned_threads_item)

        sidebar_channel_put = cls(
            thread_order=thread_order,
            hidden=hidden,
            pinned=pinned,
            pinned_threads=pinned_threads,
        )

        return sidebar_channel_put
