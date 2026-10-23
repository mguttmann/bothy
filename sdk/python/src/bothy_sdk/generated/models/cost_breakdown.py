from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.cost_total import CostTotal
    from ..models.run_cost import RunCost


T = TypeVar("T", bound="CostBreakdown")


@_attrs_define
class CostBreakdown:
    """
    Attributes:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        totals (list[CostTotal]): Valid sums per billing mode and currency, never converted; sums of mode `personal`
            only when `billing.show_personal_estimates` is on.
        runs (list[RunCost]): Runs of the thread with at least one valid cost entry, by `run_id` ascending.
    """

    thread_id: UUID
    totals: list[CostTotal]
    runs: list[RunCost]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread_id = str(self.thread_id)

        totals = []
        for totals_item_data in self.totals:
            totals_item = totals_item_data.to_dict()
            totals.append(totals_item)

        runs = []
        for runs_item_data in self.runs:
            runs_item = runs_item_data.to_dict()
            runs.append(runs_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread_id": thread_id,
                "totals": totals,
                "runs": runs,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.cost_total import CostTotal
        from ..models.run_cost import RunCost

        d = dict(src_dict)
        thread_id = UUID(d.pop("thread_id"))

        totals = []
        _totals = d.pop("totals")
        for totals_item_data in _totals:
            totals_item = CostTotal.from_dict(totals_item_data)

            totals.append(totals_item)

        runs = []
        _runs = d.pop("runs")
        for runs_item_data in _runs:
            runs_item = RunCost.from_dict(runs_item_data)

            runs.append(runs_item)

        cost_breakdown = cls(
            thread_id=thread_id,
            totals=totals,
            runs=runs,
        )

        cost_breakdown.additional_properties = d
        return cost_breakdown

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
