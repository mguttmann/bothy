from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportForumTagData")


@_attrs_define
class ImportForumTagData:
    """`data` of a `forum_tag` item (origin key `forum_tag:<channel_id>:<tag_id>`). A tag with the same name, without
    regard to case, is adopted (22, 6.6). `emoji` is accepted and not stored (forum tags have no emoji).

        Attributes:
            channel_origin_key (str):
            name (str):
            emoji (str | Unset):
    """

    channel_origin_key: str
    name: str
    emoji: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        channel_origin_key = self.channel_origin_key

        name = self.name

        emoji = self.emoji

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_origin_key": channel_origin_key,
                "name": name,
            }
        )
        if emoji is not UNSET:
            field_dict["emoji"] = emoji

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_origin_key = d.pop("channel_origin_key")

        name = d.pop("name")

        emoji = d.pop("emoji", UNSET)

        import_forum_tag_data = cls(
            channel_origin_key=channel_origin_key,
            name=name,
            emoji=emoji,
        )

        return import_forum_tag_data
