from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SearchAssistTurn")


@_attrs_define
class SearchAssistTurn:
    """
    Attributes:
        q (str): An earlier question.
        terms (list[str] | Unset): The `terms` of the answer to it.
        thread_ids (list[UUID] | Unset): The threads the answer to it showed; the assistant learns which candidates the
            person has seen already.
    """

    q: str
    terms: list[str] | Unset = UNSET
    thread_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        q = self.q

        terms: list[str] | Unset = UNSET
        if not isinstance(self.terms, Unset):
            terms = self.terms

        thread_ids: list[str] | Unset = UNSET
        if not isinstance(self.thread_ids, Unset):
            thread_ids = []
            for thread_ids_item_data in self.thread_ids:
                thread_ids_item = str(thread_ids_item_data)
                thread_ids.append(thread_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "q": q,
            }
        )
        if terms is not UNSET:
            field_dict["terms"] = terms
        if thread_ids is not UNSET:
            field_dict["thread_ids"] = thread_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        q = d.pop("q")

        terms = cast(list[str], d.pop("terms", UNSET))

        _thread_ids = d.pop("thread_ids", UNSET)
        thread_ids: list[UUID] | Unset = UNSET
        if _thread_ids is not UNSET:
            thread_ids = []
            for thread_ids_item_data in _thread_ids:
                thread_ids_item = UUID(thread_ids_item_data)

                thread_ids.append(thread_ids_item)

        search_assist_turn = cls(
            q=q,
            terms=terms,
            thread_ids=thread_ids,
        )

        return search_assist_turn
