from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="Draft")


@_attrs_define
class Draft:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        content_md (str):
        file_ids (list[UUID]):
        version (int):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        reply_to_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        updated_by_device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        updated_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    channel_id: UUID
    content_md: str
    file_ids: list[UUID]
    version: int
    thread_id: UUID | Unset = UNSET
    reply_to_message_id: UUID | Unset = UNSET
    updated_by_device_id: UUID | Unset = UNSET
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        channel_id = str(self.channel_id)

        content_md = self.content_md

        file_ids = []
        for file_ids_item_data in self.file_ids:
            file_ids_item = str(file_ids_item_data)
            file_ids.append(file_ids_item)

        version = self.version

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        reply_to_message_id: str | Unset = UNSET
        if not isinstance(self.reply_to_message_id, Unset):
            reply_to_message_id = str(self.reply_to_message_id)

        updated_by_device_id: str | Unset = UNSET
        if not isinstance(self.updated_by_device_id, Unset):
            updated_by_device_id = str(self.updated_by_device_id)

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "channel_id": channel_id,
                "content_md": content_md,
                "file_ids": file_ids,
                "version": version,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if reply_to_message_id is not UNSET:
            field_dict["reply_to_message_id"] = reply_to_message_id
        if updated_by_device_id is not UNSET:
            field_dict["updated_by_device_id"] = updated_by_device_id
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        channel_id = UUID(d.pop("channel_id"))

        content_md = d.pop("content_md")

        file_ids = []
        _file_ids = d.pop("file_ids")
        for file_ids_item_data in _file_ids:
            file_ids_item = UUID(file_ids_item_data)

            file_ids.append(file_ids_item)

        version = d.pop("version")

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _reply_to_message_id = d.pop("reply_to_message_id", UNSET)
        reply_to_message_id: UUID | Unset
        if isinstance(_reply_to_message_id, Unset):
            reply_to_message_id = UNSET
        else:
            reply_to_message_id = UUID(_reply_to_message_id)

        _updated_by_device_id = d.pop("updated_by_device_id", UNSET)
        updated_by_device_id: UUID | Unset
        if isinstance(_updated_by_device_id, Unset):
            updated_by_device_id = UNSET
        else:
            updated_by_device_id = UUID(_updated_by_device_id)

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        draft = cls(
            id=id,
            channel_id=channel_id,
            content_md=content_md,
            file_ids=file_ids,
            version=version,
            thread_id=thread_id,
            reply_to_message_id=reply_to_message_id,
            updated_by_device_id=updated_by_device_id,
            updated_at=updated_at,
        )

        draft.additional_properties = d
        return draft

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
