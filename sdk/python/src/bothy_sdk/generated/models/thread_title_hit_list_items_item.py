from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ThreadTitleHitListItemsItem")


@_attrs_define
class ThreadTitleHitListItemsItem:
    """
    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        title (str):
        last_activity_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    thread_id: UUID
    channel_id: UUID
    title: str
    last_activity_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        title = self.title

        last_activity_at = self.last_activity_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "channel_id": channel_id,
                "title": title,
                "last_activity_at": last_activity_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        title = d.pop("title")

        last_activity_at = datetime.datetime.fromisoformat(d.pop("last_activity_at"))

        thread_title_hit_list_items_item = cls(
            thread_id=thread_id,
            channel_id=channel_id,
            title=title,
            last_activity_at=last_activity_at,
        )

        thread_title_hit_list_items_item.additional_properties = d
        return thread_title_hit_list_items_item

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
