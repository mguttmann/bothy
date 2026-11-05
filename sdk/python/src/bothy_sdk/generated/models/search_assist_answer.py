from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.cost_report import CostReport
    from ..models.search_assist_pick import SearchAssistPick


T = TypeVar("T", bound="SearchAssistAnswer")


@_attrs_define
class SearchAssistAnswer:
    """Answer of the assistant agent to `search.assist_requested` (action `search.assist.respond`).

    Attributes:
        picks (list[SearchAssistPick] | Unset): The sessions that answer the question, best first; at most `max_picks`
            of the request.
        terms (list[str] | Unset): Further search terms (synonyms, the other language); at most `max_terms` of the
            request.
        costs (list[CostReport] | Unset): Usage of the model call, as in `run.finish`.
    """

    picks: list[SearchAssistPick] | Unset = UNSET
    terms: list[str] | Unset = UNSET
    costs: list[CostReport] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        picks: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.picks, Unset):
            picks = []
            for picks_item_data in self.picks:
                picks_item = picks_item_data.to_dict()
                picks.append(picks_item)

        terms: list[str] | Unset = UNSET
        if not isinstance(self.terms, Unset):
            terms = self.terms

        costs: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.costs, Unset):
            costs = []
            for costs_item_data in self.costs:
                costs_item = costs_item_data.to_dict()
                costs.append(costs_item)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if picks is not UNSET:
            field_dict["picks"] = picks
        if terms is not UNSET:
            field_dict["terms"] = terms
        if costs is not UNSET:
            field_dict["costs"] = costs

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.cost_report import CostReport
        from ..models.search_assist_pick import SearchAssistPick

        d = dict(src_dict)
        _picks = d.pop("picks", UNSET)
        picks: list[SearchAssistPick] | Unset = UNSET
        if _picks is not UNSET:
            picks = []
            for picks_item_data in _picks:
                picks_item = SearchAssistPick.from_dict(picks_item_data)

                picks.append(picks_item)

        terms = cast(list[str], d.pop("terms", UNSET))

        _costs = d.pop("costs", UNSET)
        costs: list[CostReport] | Unset = UNSET
        if _costs is not UNSET:
            costs = []
            for costs_item_data in _costs:
                costs_item = CostReport.from_dict(costs_item_data)

                costs.append(costs_item)

        search_assist_answer = cls(
            picks=picks,
            terms=terms,
            costs=costs,
        )

        return search_assist_answer
