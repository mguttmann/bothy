from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.upload_session_mode import UploadSessionMode

if TYPE_CHECKING:
    from ..models.upload_part import UploadPart


T = TypeVar("T", bound="UploadSession")


@_attrs_define
class UploadSession:
    """Upload plan. In mode `direct` the part URLs point to this server; in mode `presigned` they
    point to the object store. The client uploads all parts, then calls complete.

        Attributes:
            file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            mode (UploadSessionMode):
            part_size_bytes (int):
            parts (list[UploadPart]):
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    file_id: UUID
    mode: UploadSessionMode
    part_size_bytes: int
    parts: list[UploadPart]
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        file_id = str(self.file_id)

        mode = self.mode.value

        part_size_bytes = self.part_size_bytes

        parts = []
        for parts_item_data in self.parts:
            parts_item = parts_item_data.to_dict()
            parts.append(parts_item)

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "file_id": file_id,
                "mode": mode,
                "part_size_bytes": part_size_bytes,
                "parts": parts,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.upload_part import UploadPart

        d = dict(src_dict)
        file_id = UUID(d.pop("file_id"))

        mode = UploadSessionMode(d.pop("mode"))

        part_size_bytes = d.pop("part_size_bytes")

        parts = []
        _parts = d.pop("parts")
        for parts_item_data in _parts:
            parts_item = UploadPart.from_dict(parts_item_data)

            parts.append(parts_item)

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        upload_session = cls(
            file_id=file_id,
            mode=mode,
            part_size_bytes=part_size_bytes,
            parts=parts,
            expires_at=expires_at,
        )

        upload_session.additional_properties = d
        return upload_session

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
