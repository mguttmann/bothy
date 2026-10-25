from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.stats_timeline_bucket import StatsTimelineBucket
    from ..models.stats_timeline_run import StatsTimelineRun


T = TypeVar("T", bound="StatsTimeline")


@_attrs_define
class StatsTimeline:
    """
    Attributes:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        buckets (list[StatsTimelineBucket]):
        runs (list[StatsTimelineRun]): Runs of the visible channels whose interval from `queued_at` to `finished_at`
            (now for an unfinished run) intersects the range, ordered by `queued_at`, then `run_id`; at most
            `stats.timeline_max_runs`.
        runs_truncated (bool): True when more runs intersect the range than `runs` holds.
    """

    from_: datetime.datetime
    to: datetime.datetime
    buckets: list[StatsTimelineBucket]
    runs: list[StatsTimelineRun]
    runs_truncated: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from_ = self.from_.isoformat()

        to = self.to.isoformat()

        buckets = []
        for buckets_item_data in self.buckets:
            buckets_item = buckets_item_data.to_dict()
            buckets.append(buckets_item)

        runs = []
        for runs_item_data in self.runs:
            runs_item = runs_item_data.to_dict()
            runs.append(runs_item)

        runs_truncated = self.runs_truncated

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "from": from_,
                "to": to,
                "buckets": buckets,
                "runs": runs,
                "runs_truncated": runs_truncated,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_timeline_bucket import StatsTimelineBucket
        from ..models.stats_timeline_run import StatsTimelineRun

        d = dict(src_dict)
        from_ = datetime.datetime.fromisoformat(d.pop("from"))

        to = datetime.datetime.fromisoformat(d.pop("to"))

        buckets = []
        _buckets = d.pop("buckets")
        for buckets_item_data in _buckets:
            buckets_item = StatsTimelineBucket.from_dict(buckets_item_data)

            buckets.append(buckets_item)

        runs = []
        _runs = d.pop("runs")
        for runs_item_data in _runs:
            runs_item = StatsTimelineRun.from_dict(runs_item_data)

            runs.append(runs_item)

        runs_truncated = d.pop("runs_truncated")

        stats_timeline = cls(
            from_=from_,
            to=to,
            buckets=buckets,
            runs=runs,
            runs_truncated=runs_truncated,
        )

        stats_timeline.additional_properties = d
        return stats_timeline

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
