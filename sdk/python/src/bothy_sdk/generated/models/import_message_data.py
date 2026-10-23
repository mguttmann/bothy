from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportMessageData")


@_attrs_define
class ImportMessageData:
    """Data of a `message` item (22, 6.2). The message gets `origin = import`, `kind = text`, `silent`, no mentions and no
    `trigger_state`; `content_md` is stored as given (at most `message.max_content_bytes`, never split); the reply
    target must be a posted message of the same thread; the files must be posted file items of this source uploaded for
    the channel of the thread by the author, at most `message.max_attachments`.

        Attributes:
            author_origin_key (str):
            content_md (str):
            created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            edited_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            reply_to_origin_key (str | Unset):
            attachment_file_ids (list[UUID] | Unset):
    """

    author_origin_key: str
    content_md: str
    created_at: datetime.datetime
    edited_at: datetime.datetime | Unset = UNSET
    reply_to_origin_key: str | Unset = UNSET
    attachment_file_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        author_origin_key = self.author_origin_key

        content_md = self.content_md

        created_at = self.created_at.isoformat()

        edited_at: str | Unset = UNSET
        if not isinstance(self.edited_at, Unset):
            edited_at = self.edited_at.isoformat()

        reply_to_origin_key = self.reply_to_origin_key

        attachment_file_ids: list[str] | Unset = UNSET
        if not isinstance(self.attachment_file_ids, Unset):
            attachment_file_ids = []
            for attachment_file_ids_item_data in self.attachment_file_ids:
                attachment_file_ids_item = str(attachment_file_ids_item_data)
                attachment_file_ids.append(attachment_file_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "author_origin_key": author_origin_key,
                "content_md": content_md,
                "created_at": created_at,
            }
        )
        if edited_at is not UNSET:
            field_dict["edited_at"] = edited_at
        if reply_to_origin_key is not UNSET:
            field_dict["reply_to_origin_key"] = reply_to_origin_key
        if attachment_file_ids is not UNSET:
            field_dict["attachment_file_ids"] = attachment_file_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        author_origin_key = d.pop("author_origin_key")

        content_md = d.pop("content_md")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _edited_at = d.pop("edited_at", UNSET)
        edited_at: datetime.datetime | Unset
        if isinstance(_edited_at, Unset):
            edited_at = UNSET
        else:
            edited_at = datetime.datetime.fromisoformat(_edited_at)

        reply_to_origin_key = d.pop("reply_to_origin_key", UNSET)

        _attachment_file_ids = d.pop("attachment_file_ids", UNSET)
        attachment_file_ids: list[UUID] | Unset = UNSET
        if _attachment_file_ids is not UNSET:
            attachment_file_ids = []
            for attachment_file_ids_item_data in _attachment_file_ids:
                attachment_file_ids_item = UUID(attachment_file_ids_item_data)

                attachment_file_ids.append(attachment_file_ids_item)

        import_message_data = cls(
            author_origin_key=author_origin_key,
            content_md=content_md,
            created_at=created_at,
            edited_at=edited_at,
            reply_to_origin_key=reply_to_origin_key,
            attachment_file_ids=attachment_file_ids,
        )

        return import_message_data
