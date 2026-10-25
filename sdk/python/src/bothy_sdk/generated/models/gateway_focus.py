from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="GatewayFocus")


@_attrs_define
class GatewayFocus:
    """
    Attributes:
        thread_ids (list[UUID]): Complete set of focused threads; replaces the previous set. More than
            `sync.max_focused_threads` ids answer `error` with `validation_failed` and keep the previous set.
        idle (bool | Unset): True when the window is open but unused; live-box patches continue, but the person does not
            count as watching (13, 4.4).
        background_thread_ids (list[UUID] | Unset): Threads whose runs the app shows without their messages (control
            room cards, a run graph opened from a card). They get the run document and `run.updated` but no typing, never
            count as "Active in the thread" and are not part of the presence focus (13 4.4). Unreadable threads are ignored
            as in `thread_ids`; a thread in both lists counts as in `thread_ids`; both lists together hold at most
            `sync.max_focused_threads` distinct threads (04 6.2, R-SY-27).
    """

    thread_ids: list[UUID]
    idle: bool | Unset = UNSET
    background_thread_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        thread_ids = []
        for thread_ids_item_data in self.thread_ids:
            thread_ids_item = str(thread_ids_item_data)
            thread_ids.append(thread_ids_item)

        idle = self.idle

        background_thread_ids: list[str] | Unset = UNSET
        if not isinstance(self.background_thread_ids, Unset):
            background_thread_ids = []
            for background_thread_ids_item_data in self.background_thread_ids:
                background_thread_ids_item = str(background_thread_ids_item_data)
                background_thread_ids.append(background_thread_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "thread_ids": thread_ids,
            }
        )
        if idle is not UNSET:
            field_dict["idle"] = idle
        if background_thread_ids is not UNSET:
            field_dict["background_thread_ids"] = background_thread_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_ids = []
        _thread_ids = d.pop("thread_ids")
        for thread_ids_item_data in _thread_ids:
            thread_ids_item = UUID(thread_ids_item_data)

            thread_ids.append(thread_ids_item)

        idle = d.pop("idle", UNSET)

        _background_thread_ids = d.pop("background_thread_ids", UNSET)
        background_thread_ids: list[UUID] | Unset = UNSET
        if _background_thread_ids is not UNSET:
            background_thread_ids = []
            for background_thread_ids_item_data in _background_thread_ids:
                background_thread_ids_item = UUID(background_thread_ids_item_data)

                background_thread_ids.append(background_thread_ids_item)

        gateway_focus = cls(
            thread_ids=thread_ids,
            idle=idle,
            background_thread_ids=background_thread_ids,
        )

        return gateway_focus
