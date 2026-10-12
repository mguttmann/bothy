from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_metric import StatsMetric
from ..models.stats_slo_state import StatsSloState
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsSloEntry")


@_attrs_define
class StatsSloEntry:
    """
    Attributes:
        slo (str): `service/name` of the Sloth file.
        metric (StatsMetric): Metric of the own measurement in `stats_minutes` (docs/spec/25-visualization.md, section
            3.2).
        objective (float): Objective in percent from the Sloth file.
        period_days (int):
        total (int):
        good (int):
        state (StatsSloState): Traffic light of one SLO (docs/spec/25-visualization.md, section 6.4).
        threshold_ms (int | Unset): Threshold of a latency SLO; absent for ratio SLOs.
        ratio (float | Unset):
        budget_remaining (float | Unset): Remaining error budget, `1 - (1 - ratio) / (1 - objective/100)`; may be
            negative.
    """

    slo: str
    metric: StatsMetric
    objective: float
    period_days: int
    total: int
    good: int
    state: StatsSloState
    threshold_ms: int | Unset = UNSET
    ratio: float | Unset = UNSET
    budget_remaining: float | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        slo = self.slo

        metric = self.metric.value

        objective = self.objective

        period_days = self.period_days

        total = self.total

        good = self.good

        state = self.state.value

        threshold_ms = self.threshold_ms

        ratio = self.ratio

        budget_remaining = self.budget_remaining

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "slo": slo,
                "metric": metric,
                "objective": objective,
                "period_days": period_days,
                "total": total,
                "good": good,
                "state": state,
            }
        )
        if threshold_ms is not UNSET:
            field_dict["threshold_ms"] = threshold_ms
        if ratio is not UNSET:
            field_dict["ratio"] = ratio
        if budget_remaining is not UNSET:
            field_dict["budget_remaining"] = budget_remaining

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        slo = d.pop("slo")

        metric = StatsMetric(d.pop("metric"))

        objective = d.pop("objective")

        period_days = d.pop("period_days")

        total = d.pop("total")

        good = d.pop("good")

        state = StatsSloState(d.pop("state"))

        threshold_ms = d.pop("threshold_ms", UNSET)

        ratio = d.pop("ratio", UNSET)

        budget_remaining = d.pop("budget_remaining", UNSET)

        stats_slo_entry = cls(
            slo=slo,
            metric=metric,
            objective=objective,
            period_days=period_days,
            total=total,
            good=good,
            state=state,
            threshold_ms=threshold_ms,
            ratio=ratio,
            budget_remaining=budget_remaining,
        )

        stats_slo_entry.additional_properties = d
        return stats_slo_entry

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
