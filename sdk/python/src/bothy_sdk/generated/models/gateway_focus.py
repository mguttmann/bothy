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
    """

    thread_ids: list[UUID]
    idle: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        thread_ids = []
        for thread_ids_item_data in self.thread_ids:
            thread_ids_item = str(thread_ids_item_data)
            thread_ids.append(thread_ids_item)

        idle = self.idle

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "thread_ids": thread_ids,
            }
        )
        if idle is not UNSET:
            field_dict["idle"] = idle

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

        gateway_focus = cls(
            thread_ids=thread_ids,
            idle=idle,
        )

        return gateway_focus
