from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="RunContextAttachment")


@_attrs_define
class RunContextAttachment:
    """
    Attributes:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        original_name (str):
        mime_type (str):
        size_bytes (int):
        scan_status (str):
        transcript (str | Unset):
    """

    file_id: UUID
    original_name: str
    mime_type: str
    size_bytes: int
    scan_status: str
    transcript: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        file_id = str(self.file_id)

        original_name = self.original_name

        mime_type = self.mime_type

        size_bytes = self.size_bytes

        scan_status = self.scan_status

        transcript = self.transcript

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "file_id": file_id,
                "original_name": original_name,
                "mime_type": mime_type,
                "size_bytes": size_bytes,
                "scan_status": scan_status,
            }
        )
        if transcript is not UNSET:
            field_dict["transcript"] = transcript

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        file_id = UUID(d.pop("file_id"))

        original_name = d.pop("original_name")

        mime_type = d.pop("mime_type")

        size_bytes = d.pop("size_bytes")

        scan_status = d.pop("scan_status")

        transcript = d.pop("transcript", UNSET)

        run_context_attachment = cls(
            file_id=file_id,
            original_name=original_name,
            mime_type=mime_type,
            size_bytes=size_bytes,
            scan_status=scan_status,
            transcript=transcript,
        )

        run_context_attachment.additional_properties = d
        return run_context_attachment

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
