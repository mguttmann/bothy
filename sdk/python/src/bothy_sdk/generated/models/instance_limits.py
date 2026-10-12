from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="InstanceLimits")


@_attrs_define
class InstanceLimits:
    """Effective values of limits that clients enforce before sending.

    Attributes:
        message_max_content_bytes (int):
        message_max_attachments (int):
        file_max_upload_bytes (int):
        thread_title_max_chars (int):
        voice_max_duration_ms (int): Longest voice message (`voice.max_duration`, 12).
    """

    message_max_content_bytes: int
    message_max_attachments: int
    file_max_upload_bytes: int
    thread_title_max_chars: int
    voice_max_duration_ms: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_max_content_bytes = self.message_max_content_bytes

        message_max_attachments = self.message_max_attachments

        file_max_upload_bytes = self.file_max_upload_bytes

        thread_title_max_chars = self.thread_title_max_chars

        voice_max_duration_ms = self.voice_max_duration_ms

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_max_content_bytes": message_max_content_bytes,
                "message_max_attachments": message_max_attachments,
                "file_max_upload_bytes": file_max_upload_bytes,
                "thread_title_max_chars": thread_title_max_chars,
                "voice_max_duration_ms": voice_max_duration_ms,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_max_content_bytes = d.pop("message_max_content_bytes")

        message_max_attachments = d.pop("message_max_attachments")

        file_max_upload_bytes = d.pop("file_max_upload_bytes")

        thread_title_max_chars = d.pop("thread_title_max_chars")

        voice_max_duration_ms = d.pop("voice_max_duration_ms")

        instance_limits = cls(
            message_max_content_bytes=message_max_content_bytes,
            message_max_attachments=message_max_attachments,
            file_max_upload_bytes=file_max_upload_bytes,
            thread_title_max_chars=thread_title_max_chars,
            voice_max_duration_ms=voice_max_duration_ms,
        )

        instance_limits.additional_properties = d
        return instance_limits

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
