from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.channel_create_kind import ChannelCreateKind
from ..models.forum_sort import ForumSort
from ..types import UNSET, Unset

T = TypeVar("T", bound="ChannelCreate")


@_attrs_define
class ChannelCreate:
    """
    Attributes:
        kind (ChannelCreateKind):
        name (str):
        topic (str | Unset): At most `channel.topic_max_chars` characters, no control or bidi characters.
        forum_default_sort (ForumSort | Unset): Sort order of forum posts.
        after_channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    kind: ChannelCreateKind
    name: str
    topic: str | Unset = UNSET
    forum_default_sort: ForumSort | Unset = UNSET
    after_channel_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        name = self.name

        topic = self.topic

        forum_default_sort: str | Unset = UNSET
        if not isinstance(self.forum_default_sort, Unset):
            forum_default_sort = self.forum_default_sort.value

        after_channel_id: str | Unset = UNSET
        if not isinstance(self.after_channel_id, Unset):
            after_channel_id = str(self.after_channel_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "name": name,
            }
        )
        if topic is not UNSET:
            field_dict["topic"] = topic
        if forum_default_sort is not UNSET:
            field_dict["forum_default_sort"] = forum_default_sort
        if after_channel_id is not UNSET:
            field_dict["after_channel_id"] = after_channel_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = ChannelCreateKind(d.pop("kind"))

        name = d.pop("name")

        topic = d.pop("topic", UNSET)

        _forum_default_sort = d.pop("forum_default_sort", UNSET)
        forum_default_sort: ForumSort | Unset
        if isinstance(_forum_default_sort, Unset):
            forum_default_sort = UNSET
        else:
            forum_default_sort = ForumSort(_forum_default_sort)

        _after_channel_id = d.pop("after_channel_id", UNSET)
        after_channel_id: UUID | Unset
        if isinstance(_after_channel_id, Unset):
            after_channel_id = UNSET
        else:
            after_channel_id = UUID(_after_channel_id)

        channel_create = cls(
            kind=kind,
            name=name,
            topic=topic,
            forum_default_sort=forum_default_sort,
            after_channel_id=after_channel_id,
        )

        return channel_create
