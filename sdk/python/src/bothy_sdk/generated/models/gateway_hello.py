from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="GatewayHello")


@_attrs_define
class GatewayHello:
    """
    Attributes:
        heartbeat_interval_ms (int):
        resume_window_ms (int):
        protocol_versions (list[int]):
        dpop_nonce (str):
        server_time (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        sync_epoch (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    heartbeat_interval_ms: int
    resume_window_ms: int
    protocol_versions: list[int]
    dpop_nonce: str
    server_time: datetime.datetime
    sync_epoch: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        heartbeat_interval_ms = self.heartbeat_interval_ms

        resume_window_ms = self.resume_window_ms

        protocol_versions = self.protocol_versions

        dpop_nonce = self.dpop_nonce

        server_time = self.server_time.isoformat()

        sync_epoch = str(self.sync_epoch)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "heartbeat_interval_ms": heartbeat_interval_ms,
                "resume_window_ms": resume_window_ms,
                "protocol_versions": protocol_versions,
                "dpop_nonce": dpop_nonce,
                "server_time": server_time,
                "sync_epoch": sync_epoch,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        heartbeat_interval_ms = d.pop("heartbeat_interval_ms")

        resume_window_ms = d.pop("resume_window_ms")

        protocol_versions = cast(list[int], d.pop("protocol_versions"))

        dpop_nonce = d.pop("dpop_nonce")

        server_time = datetime.datetime.fromisoformat(d.pop("server_time"))

        sync_epoch = UUID(d.pop("sync_epoch"))

        gateway_hello = cls(
            heartbeat_interval_ms=heartbeat_interval_ms,
            resume_window_ms=resume_window_ms,
            protocol_versions=protocol_versions,
            dpop_nonce=dpop_nonce,
            server_time=server_time,
            sync_epoch=sync_epoch,
        )

        gateway_hello.additional_properties = d
        return gateway_hello

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
