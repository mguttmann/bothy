from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsRunChat")


@_attrs_define
class StatsRunChat:
    """The chat a run belongs to, as the sidebar names it: the name of its channel (the project) and the title of its
    thread. Present only when the caller may read the thread (`thread.read`, the same rule as the sidebar); absent
    otherwise, and then a client shows neither a name nor a link (docs/spec/25-visualization.md, section 4, rule 6).

        Attributes:
            thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            channel_name (str):
            thread_title (str | Unset): Title of the thread; absent for the main thread of a channel and for an empty title.
    """

    thread_id: UUID
    channel_name: str
    thread_title: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        channel_name = self.channel_name

        thread_title = self.thread_title

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "thread_id": thread_id,
                "channel_name": channel_name,
            }
        )
        if thread_title is not UNSET:
            field_dict["thread_title"] = thread_title

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        channel_name = d.pop("channel_name")

        thread_title = d.pop("thread_title", UNSET)

        stats_run_chat = cls(
            thread_id=thread_id,
            channel_name=channel_name,
            thread_title=thread_title,
        )

        return stats_run_chat
