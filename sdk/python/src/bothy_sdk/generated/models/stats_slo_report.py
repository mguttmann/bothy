from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.stats_slo_report_measured_by import StatsSloReportMeasuredBy

if TYPE_CHECKING:
    from ..models.stats_slo_entry import StatsSloEntry


T = TypeVar("T", bound="StatsSloReport")


@_attrs_define
class StatsSloReport:
    """
    Attributes:
        at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        measured_by (StatsSloReportMeasuredBy): Always `own`; the values come from the own measurement, not from
            Prometheus.
        items (list[StatsSloEntry]):
    """

    at: datetime.datetime
    measured_by: StatsSloReportMeasuredBy
    items: list[StatsSloEntry]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        at = self.at.isoformat()

        measured_by = self.measured_by.value

        items = []
        for items_item_data in self.items:
            items_item = items_item_data.to_dict()
            items.append(items_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "at": at,
                "measured_by": measured_by,
                "items": items,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_slo_entry import StatsSloEntry

        d = dict(src_dict)
        at = datetime.datetime.fromisoformat(d.pop("at"))

        measured_by = StatsSloReportMeasuredBy(d.pop("measured_by"))

        items = []
        _items = d.pop("items")
        for items_item_data in _items:
            items_item = StatsSloEntry.from_dict(items_item_data)

            items.append(items_item)

        stats_slo_report = cls(
            at=at,
            measured_by=measured_by,
            items=items,
        )

        stats_slo_report.additional_properties = d
        return stats_slo_report

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
