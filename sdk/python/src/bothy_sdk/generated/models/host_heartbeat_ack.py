from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="HostHeartbeatAck")


@_attrs_define
class HostHeartbeatAck:
    """
    Attributes:
        server_time (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    server_time: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        server_time: str | Unset = UNSET
        if not isinstance(self.server_time, Unset):
            server_time = self.server_time.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if server_time is not UNSET:
            field_dict["server_time"] = server_time

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _server_time = d.pop("server_time", UNSET)
        server_time: datetime.datetime | Unset
        if isinstance(_server_time, Unset):
            server_time = UNSET
        else:
            server_time = datetime.datetime.fromisoformat(_server_time)

        host_heartbeat_ack = cls(
            server_time=server_time,
        )

        host_heartbeat_ack.additional_properties = d
        return host_heartbeat_ack

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
