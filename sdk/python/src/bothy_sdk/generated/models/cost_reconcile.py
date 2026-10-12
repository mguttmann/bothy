from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.reconcile_entry import ReconcileEntry


T = TypeVar("T", bound="CostReconcile")


@_attrs_define
class CostReconcile:
    """
    Attributes:
        provider (str):
        period_start (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        period_end (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        entries (list[ReconcileEntry]):
    """

    provider: str
    period_start: datetime.datetime
    period_end: datetime.datetime
    entries: list[ReconcileEntry]

    def to_dict(self) -> dict[str, Any]:
        provider = self.provider

        period_start = self.period_start.isoformat()

        period_end = self.period_end.isoformat()

        entries = []
        for entries_item_data in self.entries:
            entries_item = entries_item_data.to_dict()
            entries.append(entries_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "provider": provider,
                "period_start": period_start,
                "period_end": period_end,
                "entries": entries,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.reconcile_entry import ReconcileEntry

        d = dict(src_dict)
        provider = d.pop("provider")

        period_start = datetime.datetime.fromisoformat(d.pop("period_start"))

        period_end = datetime.datetime.fromisoformat(d.pop("period_end"))

        entries = []
        _entries = d.pop("entries")
        for entries_item_data in _entries:
            entries_item = ReconcileEntry.from_dict(entries_item_data)

            entries.append(entries_item)

        cost_reconcile = cls(
            provider=provider,
            period_start=period_start,
            period_end=period_end,
            entries=entries,
        )

        return cost_reconcile
