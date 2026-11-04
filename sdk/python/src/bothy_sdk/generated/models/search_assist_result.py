from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.search_assist_result_fallback_reason import (
    SearchAssistResultFallbackReason,
)
from ..models.search_assist_result_mode import SearchAssistResultMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.search_assist_cost import SearchAssistCost
    from ..models.search_assist_hit import SearchAssistHit


T = TypeVar("T", bound="SearchAssistResult")


@_attrs_define
class SearchAssistResult:
    """
    Attributes:
        hits (list[SearchAssistHit]): At most `search.assist.max_hits`; the assistant's picks first, then full-text
            hits.
        mode (SearchAssistResultMode): `assisted` when the assistant answered, otherwise `fulltext`.
        terms (list[str]): The search terms of this question (the person's and the assistant's); the app sends them back
            in `history`.
        fallback_reason (SearchAssistResultFallbackReason | Unset): Only with `mode = fulltext`.
        cost_estimate (SearchAssistCost | Unset): Estimate of the assistant's model call, for information only; not
            booked as a cost entry.
    """

    hits: list[SearchAssistHit]
    mode: SearchAssistResultMode
    terms: list[str]
    fallback_reason: SearchAssistResultFallbackReason | Unset = UNSET
    cost_estimate: SearchAssistCost | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        hits = []
        for hits_item_data in self.hits:
            hits_item = hits_item_data.to_dict()
            hits.append(hits_item)

        mode = self.mode.value

        terms = self.terms

        fallback_reason: str | Unset = UNSET
        if not isinstance(self.fallback_reason, Unset):
            fallback_reason = self.fallback_reason.value

        cost_estimate: dict[str, Any] | Unset = UNSET
        if not isinstance(self.cost_estimate, Unset):
            cost_estimate = self.cost_estimate.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "hits": hits,
                "mode": mode,
                "terms": terms,
            }
        )
        if fallback_reason is not UNSET:
            field_dict["fallback_reason"] = fallback_reason
        if cost_estimate is not UNSET:
            field_dict["cost_estimate"] = cost_estimate

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.search_assist_cost import SearchAssistCost
        from ..models.search_assist_hit import SearchAssistHit

        d = dict(src_dict)
        hits = []
        _hits = d.pop("hits")
        for hits_item_data in _hits:
            hits_item = SearchAssistHit.from_dict(hits_item_data)

            hits.append(hits_item)

        mode = SearchAssistResultMode(d.pop("mode"))

        terms = cast(list[str], d.pop("terms"))

        _fallback_reason = d.pop("fallback_reason", UNSET)
        fallback_reason: SearchAssistResultFallbackReason | Unset
        if isinstance(_fallback_reason, Unset):
            fallback_reason = UNSET
        else:
            fallback_reason = SearchAssistResultFallbackReason(_fallback_reason)

        _cost_estimate = d.pop("cost_estimate", UNSET)
        cost_estimate: SearchAssistCost | Unset
        if isinstance(_cost_estimate, Unset):
            cost_estimate = UNSET
        else:
            cost_estimate = SearchAssistCost.from_dict(_cost_estimate)

        search_assist_result = cls(
            hits=hits,
            mode=mode,
            terms=terms,
            fallback_reason=fallback_reason,
            cost_estimate=cost_estimate,
        )

        search_assist_result.additional_properties = d
        return search_assist_result

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
