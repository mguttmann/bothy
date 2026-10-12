from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="EventScope")


@_attrs_define
class EventScope:
    """Location of the event. Members are absent where not applicable.

    Attributes:
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    space_id: UUID | Unset = UNSET
    channel_id: UUID | Unset = UNSET
    thread_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        channel_id: str | Unset = UNSET
        if not isinstance(self.channel_id, Unset):
            channel_id = str(self.channel_id)

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if channel_id is not UNSET:
            field_dict["channel_id"] = channel_id
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        _channel_id = d.pop("channel_id", UNSET)
        channel_id: UUID | Unset
        if isinstance(_channel_id, Unset):
            channel_id = UNSET
        else:
            channel_id = UUID(_channel_id)

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        event_scope = cls(
            space_id=space_id,
            channel_id=channel_id,
            thread_id=thread_id,
        )

        event_scope.additional_properties = d
        return event_scope

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
