from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="QuietHoursRuleInput")


@_attrs_define
class QuietHoursRuleInput:
    """
    Attributes:
        weekdays (int):
        start_local (str):
        end_local (str):
        enabled (bool):
        time_zone (str | Unset): IANA time zone name; absent means the person's time zone (users.time_zone).
    """

    weekdays: int
    start_local: str
    end_local: str
    enabled: bool
    time_zone: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        weekdays = self.weekdays

        start_local = self.start_local

        end_local = self.end_local

        enabled = self.enabled

        time_zone = self.time_zone

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "weekdays": weekdays,
                "start_local": start_local,
                "end_local": end_local,
                "enabled": enabled,
            }
        )
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        weekdays = d.pop("weekdays")

        start_local = d.pop("start_local")

        end_local = d.pop("end_local")

        enabled = d.pop("enabled")

        time_zone = d.pop("time_zone", UNSET)

        quiet_hours_rule_input = cls(
            weekdays=weekdays,
            start_local=start_local,
            end_local=end_local,
            enabled=enabled,
            time_zone=time_zone,
        )

        return quiet_hours_rule_input
