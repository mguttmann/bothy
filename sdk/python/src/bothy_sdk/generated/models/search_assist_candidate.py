from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="SearchAssistCandidate")


@_attrs_define
class SearchAssistCandidate:
    """
    Attributes:
        ref (str):
        channel_name (str):
        at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        excerpt (str): Plain text of the best matching message, at most `search.assist.excerpt_max_chars`.
        shown_before (bool): The person saw this session in an earlier answer of the popup.
        title (str | Unset): Thread title; absent for the main thread of a text channel.
        space_name (str | Unset):
    """

    ref: str
    channel_name: str
    at: datetime.datetime
    excerpt: str
    shown_before: bool
    title: str | Unset = UNSET
    space_name: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        ref = self.ref

        channel_name = self.channel_name

        at = self.at.isoformat()

        excerpt = self.excerpt

        shown_before = self.shown_before

        title = self.title

        space_name = self.space_name

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "ref": ref,
                "channel_name": channel_name,
                "at": at,
                "excerpt": excerpt,
                "shown_before": shown_before,
            }
        )
        if title is not UNSET:
            field_dict["title"] = title
        if space_name is not UNSET:
            field_dict["space_name"] = space_name

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        ref = d.pop("ref")

        channel_name = d.pop("channel_name")

        at = datetime.datetime.fromisoformat(d.pop("at"))

        excerpt = d.pop("excerpt")

        shown_before = d.pop("shown_before")

        title = d.pop("title", UNSET)

        space_name = d.pop("space_name", UNSET)

        search_assist_candidate = cls(
            ref=ref,
            channel_name=channel_name,
            at=at,
            excerpt=excerpt,
            shown_before=shown_before,
            title=title,
            space_name=space_name,
        )

        search_assist_candidate.additional_properties = d
        return search_assist_candidate

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
