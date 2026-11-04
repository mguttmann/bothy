from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.search_assist_turn import SearchAssistTurn


T = TypeVar("T", bound="SearchAssistQuery")


@_attrs_define
class SearchAssistQuery:
    """One question to the search assistant (14, 4.10).

    Attributes:
        q (str): The question in the person's own words, at most `search.query_max_chars`.
        history (list[SearchAssistTurn] | Unset): The earlier questions of the same popup, oldest first; at most
            `search.assist.max_turns`. Nothing of it is stored.
    """

    q: str
    history: list[SearchAssistTurn] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        q = self.q

        history: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.history, Unset):
            history = []
            for history_item_data in self.history:
                history_item = history_item_data.to_dict()
                history.append(history_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "q": q,
            }
        )
        if history is not UNSET:
            field_dict["history"] = history

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.search_assist_turn import SearchAssistTurn

        d = dict(src_dict)
        q = d.pop("q")

        _history = d.pop("history", UNSET)
        history: list[SearchAssistTurn] | Unset = UNSET
        if _history is not UNSET:
            history = []
            for history_item_data in _history:
                history_item = SearchAssistTurn.from_dict(history_item_data)

                history.append(history_item)

        search_assist_query = cls(
            q=q,
            history=history,
        )

        return search_assist_query
