from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="DeletionContents")


@_attrs_define
class DeletionContents:
    """What the deletion removes, counted when read; for a pending or final deletion what it removed or removes.

    Attributes:
        channels (int): Channels and forums, not counting dm channels.
        threads (int): Chats and threads, not counting main threads.
        messages (int):
        files (int): Files attached to those messages.
        sessions (int): Agent sessions of those chats.
    """

    channels: int
    threads: int
    messages: int
    files: int
    sessions: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channels = self.channels

        threads = self.threads

        messages = self.messages

        files = self.files

        sessions = self.sessions

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channels": channels,
                "threads": threads,
                "messages": messages,
                "files": files,
                "sessions": sessions,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channels = d.pop("channels")

        threads = d.pop("threads")

        messages = d.pop("messages")

        files = d.pop("files")

        sessions = d.pop("sessions")

        deletion_contents = cls(
            channels=channels,
            threads=threads,
            messages=messages,
            files=files,
            sessions=sessions,
        )

        deletion_contents.additional_properties = d
        return deletion_contents

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
