from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.terminal_close_reason import TerminalCloseReason

T = TypeVar("T", bound="HostSessionClose")


@_attrs_define
class HostSessionClose:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        close_reason (TerminalCloseReason): Why a session ended (24, 10.3).
    """

    session_id: UUID
    close_reason: TerminalCloseReason
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        close_reason = self.close_reason.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "close_reason": close_reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        close_reason = TerminalCloseReason(d.pop("close_reason"))

        host_session_close = cls(
            session_id=session_id,
            close_reason=close_reason,
        )

        host_session_close.additional_properties = d
        return host_session_close

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
