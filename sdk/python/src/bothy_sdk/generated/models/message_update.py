from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="MessageUpdate")


@_attrs_define
class MessageUpdate:
    """The server resolves the mentions of the resulting content again; `mention_ids` is an optional check as in
    `MessageCreate`.

        Attributes:
            content_md (str | Unset):
            mention_ids (list[UUID] | Unset):
    """

    content_md: str | Unset = UNSET
    mention_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        content_md = self.content_md

        mention_ids: list[str] | Unset = UNSET
        if not isinstance(self.mention_ids, Unset):
            mention_ids = []
            for mention_ids_item_data in self.mention_ids:
                mention_ids_item = str(mention_ids_item_data)
                mention_ids.append(mention_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if content_md is not UNSET:
            field_dict["content_md"] = content_md
        if mention_ids is not UNSET:
            field_dict["mention_ids"] = mention_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        content_md = d.pop("content_md", UNSET)

        _mention_ids = d.pop("mention_ids", UNSET)
        mention_ids: list[UUID] | Unset = UNSET
        if _mention_ids is not UNSET:
            mention_ids = []
            for mention_ids_item_data in _mention_ids:
                mention_ids_item = UUID(mention_ids_item_data)

                mention_ids.append(mention_ids_item)

        message_update = cls(
            content_md=content_md,
            mention_ids=mention_ids,
        )

        return message_update
