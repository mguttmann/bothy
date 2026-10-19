from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.jwks import Jwks


T = TypeVar("T", bound="HostChannelHello")


@_attrs_define
class HostChannelHello:
    """First text frame on the control and data connections of a host service.

    Attributes:
        protocol_versions (list[int]):
        challenge (str): Random value; the host signs it with its Ed25519 key in `identify`.
        server_time (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        heartbeat_interval_ms (int):
        heartbeat_miss_limit (int):
        jwks (Jwks): JSON Web Key Set (RFC 7517) with the public keys for access tokens, invocation tokens and terminal
            session tokens.
    """

    protocol_versions: list[int]
    challenge: str
    server_time: datetime.datetime
    heartbeat_interval_ms: int
    heartbeat_miss_limit: int
    jwks: Jwks
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        protocol_versions = self.protocol_versions

        challenge = self.challenge

        server_time = self.server_time.isoformat()

        heartbeat_interval_ms = self.heartbeat_interval_ms

        heartbeat_miss_limit = self.heartbeat_miss_limit

        jwks = self.jwks.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "protocol_versions": protocol_versions,
                "challenge": challenge,
                "server_time": server_time,
                "heartbeat_interval_ms": heartbeat_interval_ms,
                "heartbeat_miss_limit": heartbeat_miss_limit,
                "jwks": jwks,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.jwks import Jwks

        d = dict(src_dict)
        protocol_versions = cast(list[int], d.pop("protocol_versions"))

        challenge = d.pop("challenge")

        server_time = datetime.datetime.fromisoformat(d.pop("server_time"))

        heartbeat_interval_ms = d.pop("heartbeat_interval_ms")

        heartbeat_miss_limit = d.pop("heartbeat_miss_limit")

        jwks = Jwks.from_dict(d.pop("jwks"))

        host_channel_hello = cls(
            protocol_versions=protocol_versions,
            challenge=challenge,
            server_time=server_time,
            heartbeat_interval_ms=heartbeat_interval_ms,
            heartbeat_miss_limit=heartbeat_miss_limit,
            jwks=jwks,
        )

        host_channel_hello.additional_properties = d
        return host_channel_hello

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
