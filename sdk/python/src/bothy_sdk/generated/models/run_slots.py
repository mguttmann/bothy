from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="RunSlots")


@_attrs_define
class RunSlots:
    """Slot use of a runner (09 4.8, 18.2), `used` turns run of at most `total` (`runner.max_parallel_runs`) at once;
    `used` is at most `total`.

        Attributes:
            used (int):
            total (int):
    """

    used: int
    total: int

    def to_dict(self) -> dict[str, Any]:
        used = self.used

        total = self.total

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "used": used,
                "total": total,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        used = d.pop("used")

        total = d.pop("total")

        run_slots = cls(
            used=used,
            total=total,
        )

        return run_slots
