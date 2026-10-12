from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.upload_state_upload_state import UploadStateUploadState

if TYPE_CHECKING:
    from ..models.upload_state_completed_parts_item import UploadStateCompletedPartsItem


T = TypeVar("T", bound="UploadState")


@_attrs_define
class UploadState:
    """State of an own upload session, for resuming after a network loss.

    Attributes:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        upload_state (UploadStateUploadState):
        part_size_bytes (int):
        completed_parts (list[UploadStateCompletedPartsItem]): Parts the server has confirmed, with their ETag.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    file_id: UUID
    upload_state: UploadStateUploadState
    part_size_bytes: int
    completed_parts: list[UploadStateCompletedPartsItem]
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        file_id = str(self.file_id)

        upload_state = self.upload_state.value

        part_size_bytes = self.part_size_bytes

        completed_parts = []
        for completed_parts_item_data in self.completed_parts:
            completed_parts_item = completed_parts_item_data.to_dict()
            completed_parts.append(completed_parts_item)

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "file_id": file_id,
                "upload_state": upload_state,
                "part_size_bytes": part_size_bytes,
                "completed_parts": completed_parts,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.upload_state_completed_parts_item import (
            UploadStateCompletedPartsItem,
        )

        d = dict(src_dict)
        file_id = UUID(d.pop("file_id"))

        upload_state = UploadStateUploadState(d.pop("upload_state"))

        part_size_bytes = d.pop("part_size_bytes")

        completed_parts = []
        _completed_parts = d.pop("completed_parts")
        for completed_parts_item_data in _completed_parts:
            completed_parts_item = UploadStateCompletedPartsItem.from_dict(
                completed_parts_item_data
            )

            completed_parts.append(completed_parts_item)

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        upload_state = cls(
            file_id=file_id,
            upload_state=upload_state,
            part_size_bytes=part_size_bytes,
            completed_parts=completed_parts,
            expires_at=expires_at,
        )

        upload_state.additional_properties = d
        return upload_state

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
