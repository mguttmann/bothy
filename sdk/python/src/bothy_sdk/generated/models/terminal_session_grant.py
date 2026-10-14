from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.terminal_session_grant_noise_protocol import (
    TerminalSessionGrantNoiseProtocol,
)
from ..models.terminal_session_grant_prologue_version import (
    TerminalSessionGrantPrologueVersion,
)

if TYPE_CHECKING:
    from ..models.terminal_session import TerminalSession


T = TypeVar("T", bound="TerminalSessionGrant")


@_attrs_define
class TerminalSessionGrant:
    """Everything the device needs for the Noise handshake (24, 5). The token is valid once and for
    `terminal.session_token_ttl`.

        Attributes:
            session (TerminalSession): Metadata of a terminal session. Never content.
            session_token (str): Compact JWS, `typ = terminal-session+jwt`, `alg = EdDSA`.
            session_token_expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
                `Z`.
            stream_path (str):
            noise_public_key (str):
            key_fingerprint (str):
            key_generation (int):
            noise_protocol (TerminalSessionGrantNoiseProtocol):
            prologue_version (TerminalSessionGrantPrologueVersion):
    """

    session: TerminalSession
    session_token: str
    session_token_expires_at: datetime.datetime
    stream_path: str
    noise_public_key: str
    key_fingerprint: str
    key_generation: int
    noise_protocol: TerminalSessionGrantNoiseProtocol
    prologue_version: TerminalSessionGrantPrologueVersion
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session = self.session.to_dict()

        session_token = self.session_token

        session_token_expires_at = self.session_token_expires_at.isoformat()

        stream_path = self.stream_path

        noise_public_key = self.noise_public_key

        key_fingerprint = self.key_fingerprint

        key_generation = self.key_generation

        noise_protocol = self.noise_protocol.value

        prologue_version = self.prologue_version.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session": session,
                "session_token": session_token,
                "session_token_expires_at": session_token_expires_at,
                "stream_path": stream_path,
                "noise_public_key": noise_public_key,
                "key_fingerprint": key_fingerprint,
                "key_generation": key_generation,
                "noise_protocol": noise_protocol,
                "prologue_version": prologue_version,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.terminal_session import TerminalSession

        d = dict(src_dict)
        session = TerminalSession.from_dict(d.pop("session"))

        session_token = d.pop("session_token")

        session_token_expires_at = datetime.datetime.fromisoformat(
            d.pop("session_token_expires_at")
        )

        stream_path = d.pop("stream_path")

        noise_public_key = d.pop("noise_public_key")

        key_fingerprint = d.pop("key_fingerprint")

        key_generation = d.pop("key_generation")

        noise_protocol = TerminalSessionGrantNoiseProtocol(d.pop("noise_protocol"))

        prologue_version = TerminalSessionGrantPrologueVersion(
            d.pop("prologue_version")
        )

        terminal_session_grant = cls(
            session=session,
            session_token=session_token,
            session_token_expires_at=session_token_expires_at,
            stream_path=stream_path,
            noise_public_key=noise_public_key,
            key_fingerprint=key_fingerprint,
            key_generation=key_generation,
            noise_protocol=noise_protocol,
            prologue_version=prologue_version,
        )

        terminal_session_grant.additional_properties = d
        return terminal_session_grant

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
