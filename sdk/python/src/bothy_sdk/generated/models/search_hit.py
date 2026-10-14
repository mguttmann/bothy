from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.search_hit_kind import SearchHitKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.file import File
    from ..models.message import Message
    from ..models.search_hit_segments_item import SearchHitSegmentsItem
    from ..models.thread import Thread


T = TypeVar("T", bound="SearchHit")


@_attrs_define
class SearchHit:
    """One hit. `kind` names which of the optional objects is set.

    Attributes:
        kind (SearchHitKind):
        segments (list[SearchHitSegmentsItem]): Excerpt split at the matches; no markup, HTML in the content stays text
            (14, 4.6).
        score (float | Unset): Only with `sort = relevance`.
        message (Message | Unset):
        thread (Thread | Unset):
        file (File | Unset):
        thread_title (str | Unset):
        channel_name (str | Unset):
        space_name (str | Unset):
    """

    kind: SearchHitKind
    segments: list[SearchHitSegmentsItem]
    score: float | Unset = UNSET
    message: Message | Unset = UNSET
    thread: Thread | Unset = UNSET
    file: File | Unset = UNSET
    thread_title: str | Unset = UNSET
    channel_name: str | Unset = UNSET
    space_name: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        segments = []
        for segments_item_data in self.segments:
            segments_item = segments_item_data.to_dict()
            segments.append(segments_item)

        score = self.score

        message: dict[str, Any] | Unset = UNSET
        if not isinstance(self.message, Unset):
            message = self.message.to_dict()

        thread: dict[str, Any] | Unset = UNSET
        if not isinstance(self.thread, Unset):
            thread = self.thread.to_dict()

        file: dict[str, Any] | Unset = UNSET
        if not isinstance(self.file, Unset):
            file = self.file.to_dict()

        thread_title = self.thread_title

        channel_name = self.channel_name

        space_name = self.space_name

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
                "segments": segments,
            }
        )
        if score is not UNSET:
            field_dict["score"] = score
        if message is not UNSET:
            field_dict["message"] = message
        if thread is not UNSET:
            field_dict["thread"] = thread
        if file is not UNSET:
            field_dict["file"] = file
        if thread_title is not UNSET:
            field_dict["thread_title"] = thread_title
        if channel_name is not UNSET:
            field_dict["channel_name"] = channel_name
        if space_name is not UNSET:
            field_dict["space_name"] = space_name

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.file import File
        from ..models.message import Message
        from ..models.search_hit_segments_item import SearchHitSegmentsItem
        from ..models.thread import Thread

        d = dict(src_dict)
        kind = SearchHitKind(d.pop("kind"))

        segments = []
        _segments = d.pop("segments")
        for segments_item_data in _segments:
            segments_item = SearchHitSegmentsItem.from_dict(segments_item_data)

            segments.append(segments_item)

        score = d.pop("score", UNSET)

        _message = d.pop("message", UNSET)
        message: Message | Unset
        if isinstance(_message, Unset):
            message = UNSET
        else:
            message = Message.from_dict(_message)

        _thread = d.pop("thread", UNSET)
        thread: Thread | Unset
        if isinstance(_thread, Unset):
            thread = UNSET
        else:
            thread = Thread.from_dict(_thread)

        _file = d.pop("file", UNSET)
        file: File | Unset
        if isinstance(_file, Unset):
            file = UNSET
        else:
            file = File.from_dict(_file)

        thread_title = d.pop("thread_title", UNSET)

        channel_name = d.pop("channel_name", UNSET)

        space_name = d.pop("space_name", UNSET)

        search_hit = cls(
            kind=kind,
            segments=segments,
            score=score,
            message=message,
            thread=thread,
            file=file,
            thread_title=thread_title,
            channel_name=channel_name,
            space_name=space_name,
        )

        search_hit.additional_properties = d
        return search_hit

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
