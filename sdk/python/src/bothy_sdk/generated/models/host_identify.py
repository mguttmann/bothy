from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="HostIdentify")


@_attrs_define
class HostIdentify:
    """Login of a host service. `proof` is a compact JWS (`alg = EdDSA`, `typ = host-auth+jwt`, `kid`) over exactly
    `{challenge, host_id, iat}` with the registered Ed25519 key (24, 3.5).

        Attributes:
            protocol_version (int):
            host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            proof (str):
            host_version (str):
            session_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            live_sessions (list[UUID] | Unset): Only on the control connection, always present there (empty when none). The
                sessions the host service knows, those whose PTY still runs and those whose end it could not report yet (it
                reports that end with `session.ended` after `ready`). After a restart of the host service the list is empty. The
                server ends every other session of the host in state `active` or `detached` with `host_restarted` (24, 6.2).
    """

    protocol_version: int
    host_id: UUID
    proof: str
    host_version: str
    session_id: UUID | Unset = UNSET
    live_sessions: list[UUID] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        protocol_version = self.protocol_version

        host_id = str(self.host_id)

        proof = self.proof

        host_version = self.host_version

        session_id: str | Unset = UNSET
        if not isinstance(self.session_id, Unset):
            session_id = str(self.session_id)

        live_sessions: list[str] | Unset = UNSET
        if not isinstance(self.live_sessions, Unset):
            live_sessions = []
            for live_sessions_item_data in self.live_sessions:
                live_sessions_item = str(live_sessions_item_data)
                live_sessions.append(live_sessions_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "protocol_version": protocol_version,
                "host_id": host_id,
                "proof": proof,
                "host_version": host_version,
            }
        )
        if session_id is not UNSET:
            field_dict["session_id"] = session_id
        if live_sessions is not UNSET:
            field_dict["live_sessions"] = live_sessions

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        protocol_version = d.pop("protocol_version")

        host_id = UUID(d.pop("host_id"))

        proof = d.pop("proof")

        host_version = d.pop("host_version")

        _session_id = d.pop("session_id", UNSET)
        session_id: UUID | Unset
        if isinstance(_session_id, Unset):
            session_id = UNSET
        else:
            session_id = UUID(_session_id)

        _live_sessions = d.pop("live_sessions", UNSET)
        live_sessions: list[UUID] | Unset = UNSET
        if _live_sessions is not UNSET:
            live_sessions = []
            for live_sessions_item_data in _live_sessions:
                live_sessions_item = UUID(live_sessions_item_data)

                live_sessions.append(live_sessions_item)

        host_identify = cls(
            protocol_version=protocol_version,
            host_id=host_id,
            proof=proof,
            host_version=host_version,
            session_id=session_id,
            live_sessions=live_sessions,
        )

        host_identify.additional_properties = d
        return host_identify

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
