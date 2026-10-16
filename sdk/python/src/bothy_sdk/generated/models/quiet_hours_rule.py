from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="QuietHoursRule")


@_attrs_define
class QuietHoursRule:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        weekdays (int): Bit mask, bit 0 = Monday.
        start_local (str): Local time `HH:MM`.
        end_local (str):
        enabled (bool):
        version (int):
        time_zone (str | Unset): IANA time zone name; absent means the person's time zone (users.time_zone).
    """

    id: UUID
    weekdays: int
    start_local: str
    end_local: str
    enabled: bool
    version: int
    time_zone: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        weekdays = self.weekdays

        start_local = self.start_local

        end_local = self.end_local

        enabled = self.enabled

        version = self.version

        time_zone = self.time_zone

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "weekdays": weekdays,
                "start_local": start_local,
                "end_local": end_local,
                "enabled": enabled,
                "version": version,
            }
        )
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        weekdays = d.pop("weekdays")

        start_local = d.pop("start_local")

        end_local = d.pop("end_local")

        enabled = d.pop("enabled")

        version = d.pop("version")

        time_zone = d.pop("time_zone", UNSET)

        quiet_hours_rule = cls(
            id=id,
            weekdays=weekdays,
            start_local=start_local,
            end_local=end_local,
            enabled=enabled,
            version=version,
            time_zone=time_zone,
        )

        quiet_hours_rule.additional_properties = d
        return quiet_hours_rule

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
