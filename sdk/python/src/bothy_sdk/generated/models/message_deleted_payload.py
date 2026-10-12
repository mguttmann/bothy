from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="MessageDeletedPayload")


@_attrs_define
class MessageDeletedPayload:
    """
    Attributes:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_seq (int):
        deleted_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        deleted_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    message_id: UUID
    thread_id: UUID
    thread_seq: int
    deleted_at: datetime.datetime
    deleted_by: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_id = str(self.message_id)

        thread_id = str(self.thread_id)

        thread_seq = self.thread_seq

        deleted_at = self.deleted_at.isoformat()

        deleted_by: str | Unset = UNSET
        if not isinstance(self.deleted_by, Unset):
            deleted_by = str(self.deleted_by)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_id": message_id,
                "thread_id": thread_id,
                "thread_seq": thread_seq,
                "deleted_at": deleted_at,
            }
        )
        if deleted_by is not UNSET:
            field_dict["deleted_by"] = deleted_by

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_id = UUID(d.pop("message_id"))

        thread_id = UUID(d.pop("thread_id"))

        thread_seq = d.pop("thread_seq")

        deleted_at = datetime.datetime.fromisoformat(d.pop("deleted_at"))

        _deleted_by = d.pop("deleted_by", UNSET)
        deleted_by: UUID | Unset
        if isinstance(_deleted_by, Unset):
            deleted_by = UNSET
        else:
            deleted_by = UUID(_deleted_by)

        message_deleted_payload = cls(
            message_id=message_id,
            thread_id=thread_id,
            thread_seq=thread_seq,
            deleted_at=deleted_at,
            deleted_by=deleted_by,
        )

        message_deleted_payload.additional_properties = d
        return message_deleted_payload

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
