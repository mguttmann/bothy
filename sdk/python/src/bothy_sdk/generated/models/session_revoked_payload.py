from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.session_revoked_payload_reason import SessionRevokedPayloadReason

T = TypeVar("T", bound="SessionRevokedPayload")


@_attrs_define
class SessionRevokedPayload:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (SessionRevokedPayloadReason):
    """

    session_id: UUID
    reason: SessionRevokedPayloadReason
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        reason = self.reason.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        reason = SessionRevokedPayloadReason(d.pop("reason"))

        session_revoked_payload = cls(
            session_id=session_id,
            reason=reason,
        )

        session_revoked_payload.additional_properties = d
        return session_revoked_payload

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
