from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="DraftPut")


@_attrs_define
class DraftPut:
    """
    Attributes:
        content_md (str):
        reply_to_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        file_ids (list[UUID] | Unset):
        base_version (int | Unset): Version the client edited; older than current sets `overwrote_newer`.
    """

    content_md: str
    reply_to_message_id: UUID | Unset = UNSET
    file_ids: list[UUID] | Unset = UNSET
    base_version: int | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        content_md = self.content_md

        reply_to_message_id: str | Unset = UNSET
        if not isinstance(self.reply_to_message_id, Unset):
            reply_to_message_id = str(self.reply_to_message_id)

        file_ids: list[str] | Unset = UNSET
        if not isinstance(self.file_ids, Unset):
            file_ids = []
            for file_ids_item_data in self.file_ids:
                file_ids_item = str(file_ids_item_data)
                file_ids.append(file_ids_item)

        base_version = self.base_version

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "content_md": content_md,
            }
        )
        if reply_to_message_id is not UNSET:
            field_dict["reply_to_message_id"] = reply_to_message_id
        if file_ids is not UNSET:
            field_dict["file_ids"] = file_ids
        if base_version is not UNSET:
            field_dict["base_version"] = base_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        content_md = d.pop("content_md")

        _reply_to_message_id = d.pop("reply_to_message_id", UNSET)
        reply_to_message_id: UUID | Unset
        if isinstance(_reply_to_message_id, Unset):
            reply_to_message_id = UNSET
        else:
            reply_to_message_id = UUID(_reply_to_message_id)

        _file_ids = d.pop("file_ids", UNSET)
        file_ids: list[UUID] | Unset = UNSET
        if _file_ids is not UNSET:
            file_ids = []
            for file_ids_item_data in _file_ids:
                file_ids_item = UUID(file_ids_item_data)

                file_ids.append(file_ids_item)

        base_version = d.pop("base_version", UNSET)

        draft_put = cls(
            content_md=content_md,
            reply_to_message_id=reply_to_message_id,
            file_ids=file_ids,
            base_version=base_version,
        )

        return draft_put
