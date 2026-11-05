from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.search_assist_hit_source import SearchAssistHitSource
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.search_assist_hit_segments_item import SearchAssistHitSegmentsItem


T = TypeVar("T", bound="SearchAssistHit")


@_attrs_define
class SearchAssistHit:
    """One session the search found; open it at `/t/<thread_id>?m=<message_id>`.

    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_name (str): The channel, for a development session the project forum.
        at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        source (SearchAssistHitSource):
        segments (list[SearchAssistHitSegmentsItem]): Excerpt split at the matches, as in `SearchHit`.
        message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_title (str | Unset): Absent for the main thread of a text channel.
        space_name (str | Unset): Absent for a direct message.
        reason (str | Unset): One sentence of the assistant why this is the session; only with `source = assistant`.
    """

    thread_id: UUID
    channel_id: UUID
    channel_name: str
    at: datetime.datetime
    source: SearchAssistHitSource
    segments: list[SearchAssistHitSegmentsItem]
    message_id: UUID | Unset = UNSET
    thread_title: str | Unset = UNSET
    space_name: str | Unset = UNSET
    reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        channel_name = self.channel_name

        at = self.at.isoformat()

        source = self.source.value

        segments = []
        for segments_item_data in self.segments:
            segments_item = segments_item_data.to_dict()
            segments.append(segments_item)

        message_id: str | Unset = UNSET
        if not isinstance(self.message_id, Unset):
            message_id = str(self.message_id)

        thread_title = self.thread_title

        space_name = self.space_name

        reason = self.reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "channel_id": channel_id,
                "channel_name": channel_name,
                "at": at,
                "source": source,
                "segments": segments,
            }
        )
        if message_id is not UNSET:
            field_dict["message_id"] = message_id
        if thread_title is not UNSET:
            field_dict["thread_title"] = thread_title
        if space_name is not UNSET:
            field_dict["space_name"] = space_name
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.search_assist_hit_segments_item import SearchAssistHitSegmentsItem

        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        channel_name = d.pop("channel_name")

        at = datetime.datetime.fromisoformat(d.pop("at"))

        source = SearchAssistHitSource(d.pop("source"))

        segments = []
        _segments = d.pop("segments")
        for segments_item_data in _segments:
            segments_item = SearchAssistHitSegmentsItem.from_dict(segments_item_data)

            segments.append(segments_item)

        _message_id = d.pop("message_id", UNSET)
        message_id: UUID | Unset
        if isinstance(_message_id, Unset):
            message_id = UNSET
        else:
            message_id = UUID(_message_id)

        thread_title = d.pop("thread_title", UNSET)

        space_name = d.pop("space_name", UNSET)

        reason = d.pop("reason", UNSET)

        search_assist_hit = cls(
            thread_id=thread_id,
            channel_id=channel_id,
            channel_name=channel_name,
            at=at,
            source=source,
            segments=segments,
            message_id=message_id,
            thread_title=thread_title,
            space_name=space_name,
            reason=reason,
        )

        search_assist_hit.additional_properties = d
        return search_assist_hit

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
