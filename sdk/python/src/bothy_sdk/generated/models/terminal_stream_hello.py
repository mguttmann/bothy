from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="TerminalStreamHello")


@_attrs_define
class TerminalStreamHello:
    """First text frame of the device stream; the binary Noise frames follow `identify`.

    Attributes:
        protocol_versions (list[int]):
        dpop_nonce (str):
        server_time (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    protocol_versions: list[int]
    dpop_nonce: str
    server_time: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        protocol_versions = self.protocol_versions

        dpop_nonce = self.dpop_nonce

        server_time = self.server_time.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "protocol_versions": protocol_versions,
                "dpop_nonce": dpop_nonce,
                "server_time": server_time,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        protocol_versions = cast(list[int], d.pop("protocol_versions"))

        dpop_nonce = d.pop("dpop_nonce")

        server_time = datetime.datetime.fromisoformat(d.pop("server_time"))

        terminal_stream_hello = cls(
            protocol_versions=protocol_versions,
            dpop_nonce=dpop_nonce,
            server_time=server_time,
        )

        terminal_stream_hello.additional_properties = d
        return terminal_stream_hello

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
