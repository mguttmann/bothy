from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="TerminalStreamIdentify")


@_attrs_define
class TerminalStreamIdentify:
    """Native apps send an access token with DPoP proof (`htm = GET`, `htu` = HTTPS form of the stream URL, `nonce` from
    `hello`, `ath`). The web console sends neither: its stream authenticates with the console session cookie of the
    upgrade request and the same-origin check of the upgrade (docs/spec/08-identity.md section 7.5, docs/spec/15-client-
    architecture.md section 23.2); the server rejects an identify without token from any other session. Only the device
    of the current session token passes (`terminal.stream`).

        Attributes:
            protocol_version (int):
            device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            access_token (str | Unset):
            dpop (str | Unset):
    """

    protocol_version: int
    device_id: UUID
    access_token: str | Unset = UNSET
    dpop: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        protocol_version = self.protocol_version

        device_id = str(self.device_id)

        access_token = self.access_token

        dpop = self.dpop

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "protocol_version": protocol_version,
                "device_id": device_id,
            }
        )
        if access_token is not UNSET:
            field_dict["access_token"] = access_token
        if dpop is not UNSET:
            field_dict["dpop"] = dpop

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        protocol_version = d.pop("protocol_version")

        device_id = UUID(d.pop("device_id"))

        access_token = d.pop("access_token", UNSET)

        dpop = d.pop("dpop", UNSET)

        terminal_stream_identify = cls(
            protocol_version=protocol_version,
            device_id=device_id,
            access_token=access_token,
            dpop=dpop,
        )

        terminal_stream_identify.additional_properties = d
        return terminal_stream_identify

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
