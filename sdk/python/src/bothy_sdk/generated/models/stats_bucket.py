from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.stats_bucket_metrics import StatsBucketMetrics


T = TypeVar("T", bound="StatsBucket")


@_attrs_define
class StatsBucket:
    """One time bucket. `group_key` is set when the query groups (space, agent, tool).

    Attributes:
        start (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        metrics (StatsBucketMetrics): Metric name to value. Names per endpoint are listed in its description.
        group_key (str | Unset):
    """

    start: datetime.datetime
    metrics: StatsBucketMetrics
    group_key: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        start = self.start.isoformat()

        metrics = self.metrics.to_dict()

        group_key = self.group_key

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "start": start,
                "metrics": metrics,
            }
        )
        if group_key is not UNSET:
            field_dict["group_key"] = group_key

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_bucket_metrics import StatsBucketMetrics

        d = dict(src_dict)
        start = datetime.datetime.fromisoformat(d.pop("start"))

        metrics = StatsBucketMetrics.from_dict(d.pop("metrics"))

        group_key = d.pop("group_key", UNSET)

        stats_bucket = cls(
            start=start,
            metrics=metrics,
            group_key=group_key,
        )

        stats_bucket.additional_properties = d
        return stats_bucket

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
