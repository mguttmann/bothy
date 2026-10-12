from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_granularity import StatsGranularity

if TYPE_CHECKING:
    from ..models.cost_bucket import CostBucket


T = TypeVar("T", bound="CostSeries")


@_attrs_define
class CostSeries:
    """
    Attributes:
        granularity (StatsGranularity): Size of a statistics bucket.
        currency (str):
        buckets (list[CostBucket]):
    """

    granularity: StatsGranularity
    currency: str
    buckets: list[CostBucket]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        granularity = self.granularity.value

        currency = self.currency

        buckets = []
        for buckets_item_data in self.buckets:
            buckets_item = buckets_item_data.to_dict()
            buckets.append(buckets_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "granularity": granularity,
                "currency": currency,
                "buckets": buckets,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.cost_bucket import CostBucket

        d = dict(src_dict)
        granularity = StatsGranularity(d.pop("granularity"))

        currency = d.pop("currency")

        buckets = []
        _buckets = d.pop("buckets")
        for buckets_item_data in _buckets:
            buckets_item = CostBucket.from_dict(buckets_item_data)

            buckets.append(buckets_item)

        cost_series = cls(
            granularity=granularity,
            currency=currency,
            buckets=buckets,
        )

        cost_series.additional_properties = d
        return cost_series

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
