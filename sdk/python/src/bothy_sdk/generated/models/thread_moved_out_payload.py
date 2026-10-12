from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ThreadMovedOutPayload")


@_attrs_define
class ThreadMovedOutPayload:
    """Sent in the source channel. Does not name the target channel.

    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    thread_id: UUID
    channel_id: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "channel_id": channel_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        thread_moved_out_payload = cls(
            thread_id=thread_id,
            channel_id=channel_id,
        )

        thread_moved_out_payload.additional_properties = d
        return thread_moved_out_payload

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
