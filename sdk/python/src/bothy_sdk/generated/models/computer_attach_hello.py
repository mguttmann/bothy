from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ComputerAttachHello")


@_attrs_define
class ComputerAttachHello:
    """First text frame of the computer attach connection; `identify` follows, then binary Noise frames only.

    Attributes:
        protocol_versions (list[int]): Versions of the attach protocol the server speaks.
        server_time (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    protocol_versions: list[int]
    server_time: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        protocol_versions = self.protocol_versions

        server_time = self.server_time.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "protocol_versions": protocol_versions,
                "server_time": server_time,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        protocol_versions = cast(list[int], d.pop("protocol_versions"))

        server_time = datetime.datetime.fromisoformat(d.pop("server_time"))

        computer_attach_hello = cls(
            protocol_versions=protocol_versions,
            server_time=server_time,
        )

        computer_attach_hello.additional_properties = d
        return computer_attach_hello

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
