from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportThreadData")


@_attrs_define
class ImportThreadData:
    """Data of a `thread` item (22, 5.1.1). The title is `name` cut to `thread.title_max_chars`, `title_source` is `auto`
    for a placeholder name and `manual` otherwise; the operator is the planned operator of the channel in the mapping;
    `parent_message_id` is the starter message only when its item is posted in the main thread of the same channel; the
    tags are the posted `forum_tag` items of the same channel. A posted thread only follows the mirror rules of 22 12.2
    step 3.

        Attributes:
            channel_origin_key (str):
            name (str):
            created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            last_activity_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            closed (bool): The archive rule of the tool (22 5.1.1, R-IM-11).
            closed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            starter_message_origin_key (str | Unset):
            tag_origin_keys (list[str] | Unset):
    """

    channel_origin_key: str
    name: str
    created_at: datetime.datetime
    last_activity_at: datetime.datetime
    closed: bool
    closed_at: datetime.datetime | Unset = UNSET
    starter_message_origin_key: str | Unset = UNSET
    tag_origin_keys: list[str] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        channel_origin_key = self.channel_origin_key

        name = self.name

        created_at = self.created_at.isoformat()

        last_activity_at = self.last_activity_at.isoformat()

        closed = self.closed

        closed_at: str | Unset = UNSET
        if not isinstance(self.closed_at, Unset):
            closed_at = self.closed_at.isoformat()

        starter_message_origin_key = self.starter_message_origin_key

        tag_origin_keys: list[str] | Unset = UNSET
        if not isinstance(self.tag_origin_keys, Unset):
            tag_origin_keys = self.tag_origin_keys

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "channel_origin_key": channel_origin_key,
                "name": name,
                "created_at": created_at,
                "last_activity_at": last_activity_at,
                "closed": closed,
            }
        )
        if closed_at is not UNSET:
            field_dict["closed_at"] = closed_at
        if starter_message_origin_key is not UNSET:
            field_dict["starter_message_origin_key"] = starter_message_origin_key
        if tag_origin_keys is not UNSET:
            field_dict["tag_origin_keys"] = tag_origin_keys

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_origin_key = d.pop("channel_origin_key")

        name = d.pop("name")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        last_activity_at = datetime.datetime.fromisoformat(d.pop("last_activity_at"))

        closed = d.pop("closed")

        _closed_at = d.pop("closed_at", UNSET)
        closed_at: datetime.datetime | Unset
        if isinstance(_closed_at, Unset):
            closed_at = UNSET
        else:
            closed_at = datetime.datetime.fromisoformat(_closed_at)

        starter_message_origin_key = d.pop("starter_message_origin_key", UNSET)

        tag_origin_keys = cast(list[str], d.pop("tag_origin_keys", UNSET))

        import_thread_data = cls(
            channel_origin_key=channel_origin_key,
            name=name,
            created_at=created_at,
            last_activity_at=last_activity_at,
            closed=closed,
            closed_at=closed_at,
            starter_message_origin_key=starter_message_origin_key,
            tag_origin_keys=tag_origin_keys,
        )

        return import_thread_data
