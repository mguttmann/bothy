from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

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
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_ids = []
        for thread_ids_item_data in self.thread_ids:
            thread_ids_item = str(thread_ids_item_data)
            thread_ids.append(thread_ids_item)

        idle = self.idle

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
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

        gateway_focus.additional_properties = d
        return gateway_focus

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
