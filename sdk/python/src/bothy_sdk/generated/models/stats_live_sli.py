from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_metric import StatsMetric
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsLiveSli")


@_attrs_define
class StatsLiveSli:
    """
    Attributes:
        metric (StatsMetric): Metric of the own measurement in `stats_minutes` (docs/spec/25-visualization.md, section
            3.2).
        total (int):
        good (int):
        ratio (float | Unset): `good / total`; absent when `total` is 0.
    """

    metric: StatsMetric
    total: int
    good: int
    ratio: float | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        metric = self.metric.value

        total = self.total

        good = self.good

        ratio = self.ratio

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "metric": metric,
                "total": total,
                "good": good,
            }
        )
        if ratio is not UNSET:
            field_dict["ratio"] = ratio

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        metric = StatsMetric(d.pop("metric"))

        total = d.pop("total")

        good = d.pop("good")

        ratio = d.pop("ratio", UNSET)

        stats_live_sli = cls(
            metric=metric,
            total=total,
            good=good,
            ratio=ratio,
        )

        stats_live_sli.additional_properties = d
        return stats_live_sli

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
