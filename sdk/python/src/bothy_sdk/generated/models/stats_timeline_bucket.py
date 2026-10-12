from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_metric import StatsMetric
from ..types import UNSET, Unset

T = TypeVar("T", bound="StatsTimelineBucket")


@_attrs_define
class StatsTimelineBucket:
    """One row of `stats_minutes`.

    Attributes:
        start (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        metric (StatsMetric): Metric of the own measurement in `stats_minutes` (docs/spec/25-visualisierung.md, section
            3.2).
        dimension (str): Code from the closed list per metric; empty string when the metric has none.
        count (int):
        sum_ (int):
        good (int):
        channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        currency (str | Unset):
    """

    start: datetime.datetime
    metric: StatsMetric
    dimension: str
    count: int
    sum_: int
    good: int
    channel_id: UUID | Unset = UNSET
    agent_id: UUID | Unset = UNSET
    currency: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        start = self.start.isoformat()

        metric = self.metric.value

        dimension = self.dimension

        count = self.count

        sum_ = self.sum_

        good = self.good

        channel_id: str | Unset = UNSET
        if not isinstance(self.channel_id, Unset):
            channel_id = str(self.channel_id)

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        currency = self.currency

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "start": start,
                "metric": metric,
                "dimension": dimension,
                "count": count,
                "sum": sum_,
                "good": good,
            }
        )
        if channel_id is not UNSET:
            field_dict["channel_id"] = channel_id
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if currency is not UNSET:
            field_dict["currency"] = currency

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        start = datetime.datetime.fromisoformat(d.pop("start"))

        metric = StatsMetric(d.pop("metric"))

        dimension = d.pop("dimension")

        count = d.pop("count")

        sum_ = d.pop("sum")

        good = d.pop("good")

        _channel_id = d.pop("channel_id", UNSET)
        channel_id: UUID | Unset
        if isinstance(_channel_id, Unset):
            channel_id = UNSET
        else:
            channel_id = UUID(_channel_id)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        currency = d.pop("currency", UNSET)

        stats_timeline_bucket = cls(
            start=start,
            metric=metric,
            dimension=dimension,
            count=count,
            sum_=sum_,
            good=good,
            channel_id=channel_id,
            agent_id=agent_id,
            currency=currency,
        )

        stats_timeline_bucket.additional_properties = d
        return stats_timeline_bucket

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
