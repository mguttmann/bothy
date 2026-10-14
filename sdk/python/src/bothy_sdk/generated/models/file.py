from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.file_purpose import FilePurpose
from ..models.file_scan_status import FileScanStatus
from ..models.file_upload_state import FileUploadState
from ..models.transcript_source import TranscriptSource
from ..models.transcript_status import TranscriptStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.thumbnail import Thumbnail


T = TypeVar("T", bound="File")


@_attrs_define
class File:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        size_bytes (int):
        mime_type (str): Detected by the server.
        original_name (str):
        uploaded_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        purpose (FilePurpose):
        upload_state (FileUploadState):
        scan_status (FileScanStatus):
        transcript_status (TranscriptStatus):
        thumbnails (list[Thumbnail]):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        sha256 (str | Unset): Hex SHA-256 of the stored bytes.
        scope_channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        width (int | Unset):
        height (int | Unset):
        duration_ms (int | Unset):
        transcript_language (str | Unset):
        transcript_source (TranscriptSource | Unset):
    """

    id: UUID
    size_bytes: int
    mime_type: str
    original_name: str
    uploaded_by: UUID
    purpose: FilePurpose
    upload_state: FileUploadState
    scan_status: FileScanStatus
    transcript_status: TranscriptStatus
    thumbnails: list[Thumbnail]
    created_at: datetime.datetime
    sha256: str | Unset = UNSET
    scope_channel_id: UUID | Unset = UNSET
    width: int | Unset = UNSET
    height: int | Unset = UNSET
    duration_ms: int | Unset = UNSET
    transcript_language: str | Unset = UNSET
    transcript_source: TranscriptSource | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        size_bytes = self.size_bytes

        mime_type = self.mime_type

        original_name = self.original_name

        uploaded_by = str(self.uploaded_by)

        purpose = self.purpose.value

        upload_state = self.upload_state.value

        scan_status = self.scan_status.value

        transcript_status = self.transcript_status.value

        thumbnails = []
        for thumbnails_item_data in self.thumbnails:
            thumbnails_item = thumbnails_item_data.to_dict()
            thumbnails.append(thumbnails_item)

        created_at = self.created_at.isoformat()

        sha256 = self.sha256

        scope_channel_id: str | Unset = UNSET
        if not isinstance(self.scope_channel_id, Unset):
            scope_channel_id = str(self.scope_channel_id)

        width = self.width

        height = self.height

        duration_ms = self.duration_ms

        transcript_language = self.transcript_language

        transcript_source: str | Unset = UNSET
        if not isinstance(self.transcript_source, Unset):
            transcript_source = self.transcript_source.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "size_bytes": size_bytes,
                "mime_type": mime_type,
                "original_name": original_name,
                "uploaded_by": uploaded_by,
                "purpose": purpose,
                "upload_state": upload_state,
                "scan_status": scan_status,
                "transcript_status": transcript_status,
                "thumbnails": thumbnails,
                "created_at": created_at,
            }
        )
        if sha256 is not UNSET:
            field_dict["sha256"] = sha256
        if scope_channel_id is not UNSET:
            field_dict["scope_channel_id"] = scope_channel_id
        if width is not UNSET:
            field_dict["width"] = width
        if height is not UNSET:
            field_dict["height"] = height
        if duration_ms is not UNSET:
            field_dict["duration_ms"] = duration_ms
        if transcript_language is not UNSET:
            field_dict["transcript_language"] = transcript_language
        if transcript_source is not UNSET:
            field_dict["transcript_source"] = transcript_source

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.thumbnail import Thumbnail

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        size_bytes = d.pop("size_bytes")

        mime_type = d.pop("mime_type")

        original_name = d.pop("original_name")

        uploaded_by = UUID(d.pop("uploaded_by"))

        purpose = FilePurpose(d.pop("purpose"))

        upload_state = FileUploadState(d.pop("upload_state"))

        scan_status = FileScanStatus(d.pop("scan_status"))

        transcript_status = TranscriptStatus(d.pop("transcript_status"))

        thumbnails = []
        _thumbnails = d.pop("thumbnails")
        for thumbnails_item_data in _thumbnails:
            thumbnails_item = Thumbnail.from_dict(thumbnails_item_data)

            thumbnails.append(thumbnails_item)

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        sha256 = d.pop("sha256", UNSET)

        _scope_channel_id = d.pop("scope_channel_id", UNSET)
        scope_channel_id: UUID | Unset
        if isinstance(_scope_channel_id, Unset):
            scope_channel_id = UNSET
        else:
            scope_channel_id = UUID(_scope_channel_id)

        width = d.pop("width", UNSET)

        height = d.pop("height", UNSET)

        duration_ms = d.pop("duration_ms", UNSET)

        transcript_language = d.pop("transcript_language", UNSET)

        _transcript_source = d.pop("transcript_source", UNSET)
        transcript_source: TranscriptSource | Unset
        if isinstance(_transcript_source, Unset):
            transcript_source = UNSET
        else:
            transcript_source = TranscriptSource(_transcript_source)

        file = cls(
            id=id,
            size_bytes=size_bytes,
            mime_type=mime_type,
            original_name=original_name,
            uploaded_by=uploaded_by,
            purpose=purpose,
            upload_state=upload_state,
            scan_status=scan_status,
            transcript_status=transcript_status,
            thumbnails=thumbnails,
            created_at=created_at,
            sha256=sha256,
            scope_channel_id=scope_channel_id,
            width=width,
            height=height,
            duration_ms=duration_ms,
            transcript_language=transcript_language,
            transcript_source=transcript_source,
        )

        file.additional_properties = d
        return file

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
