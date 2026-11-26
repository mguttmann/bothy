from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ComputerSessionOpen")


@_attrs_define
class ComputerSessionOpen:
    """
    Attributes:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        client_ephemeral_key_hash (str): SHA-256 of the ephemeral X25519 public key that `bothy-computer-mcp` sends in
            Noise message 1, base64url without padding; becomes the claim `cek`.
        pin_changed (bool | Unset): `true` when `bothy-computer-mcp` replaced its pin of the host key after a re-pairing
            (27 10.4); recorded in the audit entry of `computer.open`. Default: False.
    """

    host_id: UUID
    client_ephemeral_key_hash: str
    pin_changed: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        host_id = str(self.host_id)

        client_ephemeral_key_hash = self.client_ephemeral_key_hash

        pin_changed = self.pin_changed

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "host_id": host_id,
                "client_ephemeral_key_hash": client_ephemeral_key_hash,
            }
        )
        if pin_changed is not UNSET:
            field_dict["pin_changed"] = pin_changed

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        host_id = UUID(d.pop("host_id"))

        client_ephemeral_key_hash = d.pop("client_ephemeral_key_hash")

        pin_changed = d.pop("pin_changed", UNSET)

        computer_session_open = cls(
            host_id=host_id,
            client_ephemeral_key_hash=client_ephemeral_key_hash,
            pin_changed=pin_changed,
        )

        return computer_session_open
