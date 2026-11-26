from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ComputerSessionGrant")


@_attrs_define
class ComputerSessionGrant:
    """Everything `bothy-computer-mcp` needs to attach and run the Noise handshake (27 5.1 step 6). The token is valid once
    and for `computer.attach_token_ttl`.

        Attributes:
            session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            token (str): Compact JWS, `typ = computer-session+jwt`, `alg = EdDSA`, with the claims of 27 5.2.
            attach_path (str): Path of the attach connection (`computer.attach`).
            noise_public_key (str): The static Noise public key of the host that `bothy-computer-mcp` checks in the
                handshake (27 10.4).
            key_fingerprint (str): Fingerprint of `noise_public_key`.
            key_generation (int): Generation of the host key.
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    session_id: UUID
    token: str
    attach_path: str
    noise_public_key: str
    key_fingerprint: str
    key_generation: int
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        token = self.token

        attach_path = self.attach_path

        noise_public_key = self.noise_public_key

        key_fingerprint = self.key_fingerprint

        key_generation = self.key_generation

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "token": token,
                "attach_path": attach_path,
                "noise_public_key": noise_public_key,
                "key_fingerprint": key_fingerprint,
                "key_generation": key_generation,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        token = d.pop("token")

        attach_path = d.pop("attach_path")

        noise_public_key = d.pop("noise_public_key")

        key_fingerprint = d.pop("key_fingerprint")

        key_generation = d.pop("key_generation")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        computer_session_grant = cls(
            session_id=session_id,
            token=token,
            attach_path=attach_path,
            noise_public_key=noise_public_key,
            key_fingerprint=key_fingerprint,
            key_generation=key_generation,
            expires_at=expires_at,
        )

        computer_session_grant.additional_properties = d
        return computer_session_grant

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
