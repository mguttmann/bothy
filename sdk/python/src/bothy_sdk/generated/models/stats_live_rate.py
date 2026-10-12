from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_metric import StatsMetric
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsLiveRate")


@_attrs_define
class StatsLiveRate:
    """Sum of `count` (and `sum` where meaningful) of one metric over the last full minute and the current minute up to
    `at`.

        Attributes:
            metric (StatsMetric): Metric of the own measurement in `stats_minutes` (docs/spec/25-visualisierung.md, section
                3.2).
            last_minute (int):
            current_minute (int):
            dimension (str | Unset): Dimension code; absent means all dimensions summed.
            last_minute_sum (int | Unset): Sum of values (milliseconds or micro units) of the last full minute.
            currency (str | Unset): Only for metric `cost`.
    """

    metric: StatsMetric
    last_minute: int
    current_minute: int
    dimension: str | Unset = UNSET
    last_minute_sum: int | Unset = UNSET
    currency: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        metric = self.metric.value

        last_minute = self.last_minute

        current_minute = self.current_minute

        dimension = self.dimension

        last_minute_sum = self.last_minute_sum

        currency = self.currency

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "metric": metric,
                "last_minute": last_minute,
                "current_minute": current_minute,
            }
        )
        if dimension is not UNSET:
            field_dict["dimension"] = dimension
        if last_minute_sum is not UNSET:
            field_dict["last_minute_sum"] = last_minute_sum
        if currency is not UNSET:
            field_dict["currency"] = currency

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        metric = StatsMetric(d.pop("metric"))

        last_minute = d.pop("last_minute")

        current_minute = d.pop("current_minute")

        dimension = d.pop("dimension", UNSET)

        last_minute_sum = d.pop("last_minute_sum", UNSET)

        currency = d.pop("currency", UNSET)

        stats_live_rate = cls(
            metric=metric,
            last_minute=last_minute,
            current_minute=current_minute,
            dimension=dimension,
            last_minute_sum=last_minute_sum,
            currency=currency,
        )

        stats_live_rate.additional_properties = d
        return stats_live_rate

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
