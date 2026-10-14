from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_granularity import StatsGranularity

if TYPE_CHECKING:
    from ..models.stats_bucket import StatsBucket


T = TypeVar("T", bound="StatsSeries")


@_attrs_define
class StatsSeries:
    """
    Attributes:
        granularity (StatsGranularity): Size of a statistics bucket.
        buckets (list[StatsBucket]):
    """

    granularity: StatsGranularity
    buckets: list[StatsBucket]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        granularity = self.granularity.value

        buckets = []
        for buckets_item_data in self.buckets:
            buckets_item = buckets_item_data.to_dict()
            buckets.append(buckets_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "granularity": granularity,
                "buckets": buckets,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_bucket import StatsBucket

        d = dict(src_dict)
        granularity = StatsGranularity(d.pop("granularity"))

        buckets = []
        _buckets = d.pop("buckets")
        for buckets_item_data in _buckets:
            buckets_item = StatsBucket.from_dict(buckets_item_data)

            buckets.append(buckets_item)

        stats_series = cls(
            granularity=granularity,
            buckets=buckets,
        )

        stats_series.additional_properties = d
        return stats_series

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
