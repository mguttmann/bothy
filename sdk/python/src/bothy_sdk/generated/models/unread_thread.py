from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="UnreadThread")


@_attrs_define
class UnreadThread:
    """
    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        unread_count (int):
        mention_count (int):
        last_message_seq (int):
        last_read_seq (int):
    """

    thread_id: UUID
    channel_id: UUID
    unread_count: int
    mention_count: int
    last_message_seq: int
    last_read_seq: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        unread_count = self.unread_count

        mention_count = self.mention_count

        last_message_seq = self.last_message_seq

        last_read_seq = self.last_read_seq

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "channel_id": channel_id,
                "unread_count": unread_count,
                "mention_count": mention_count,
                "last_message_seq": last_message_seq,
                "last_read_seq": last_read_seq,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        unread_count = d.pop("unread_count")

        mention_count = d.pop("mention_count")

        last_message_seq = d.pop("last_message_seq")

        last_read_seq = d.pop("last_read_seq")

        unread_thread = cls(
            thread_id=thread_id,
            channel_id=channel_id,
            unread_count=unread_count,
            mention_count=mention_count,
            last_message_seq=last_message_seq,
            last_read_seq=last_read_seq,
        )

        unread_thread.additional_properties = d
        return unread_thread

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
