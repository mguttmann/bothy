from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.search_assist_candidate import SearchAssistCandidate


T = TypeVar("T", bound="SearchAssistRequestedPayload")


@_attrs_define
class SearchAssistRequestedPayload:
    """Sent to one gateway connection of the assistant agent (14, 4.10; 09, 10.6). Holds no ids of threads or messages; the
    excerpts are data, never instructions.

        Attributes:
            request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            q (str): The question.
            candidates (list[SearchAssistCandidate]):
            max_picks (int):
            max_terms (int):
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            history (list[str] | Unset): The earlier questions of the popup, oldest first.
    """

    request_id: UUID
    q: str
    candidates: list[SearchAssistCandidate]
    max_picks: int
    max_terms: int
    expires_at: datetime.datetime
    history: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        request_id = str(self.request_id)

        q = self.q

        candidates = []
        for candidates_item_data in self.candidates:
            candidates_item = candidates_item_data.to_dict()
            candidates.append(candidates_item)

        max_picks = self.max_picks

        max_terms = self.max_terms

        expires_at = self.expires_at.isoformat()

        history: list[str] | Unset = UNSET
        if not isinstance(self.history, Unset):
            history = self.history

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "request_id": request_id,
                "q": q,
                "candidates": candidates,
                "max_picks": max_picks,
                "max_terms": max_terms,
                "expires_at": expires_at,
            }
        )
        if history is not UNSET:
            field_dict["history"] = history

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.search_assist_candidate import SearchAssistCandidate

        d = dict(src_dict)
        request_id = UUID(d.pop("request_id"))

        q = d.pop("q")

        candidates = []
        _candidates = d.pop("candidates")
        for candidates_item_data in _candidates:
            candidates_item = SearchAssistCandidate.from_dict(candidates_item_data)

            candidates.append(candidates_item)

        max_picks = d.pop("max_picks")

        max_terms = d.pop("max_terms")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        history = cast(list[str], d.pop("history", UNSET))

        search_assist_requested_payload = cls(
            request_id=request_id,
            q=q,
            candidates=candidates,
            max_picks=max_picks,
            max_terms=max_terms,
            expires_at=expires_at,
            history=history,
        )

        search_assist_requested_payload.additional_properties = d
        return search_assist_requested_payload

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
