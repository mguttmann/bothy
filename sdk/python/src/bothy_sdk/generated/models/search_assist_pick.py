from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="SearchAssistPick")


@_attrs_define
class SearchAssistPick:
    """
    Attributes:
        ref (str): The `ref` of a candidate of the request.
        reason (str): One sentence why, at most `search.assist.reason_max_chars`.
    """

    ref: str
    reason: str

    def to_dict(self) -> dict[str, Any]:
        ref = self.ref

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "ref": ref,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        ref = d.pop("ref")

        reason = d.pop("reason")

        search_assist_pick = cls(
            ref=ref,
            reason=reason,
        )

        return search_assist_pick
