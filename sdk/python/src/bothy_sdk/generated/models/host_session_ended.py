from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.terminal_close_detail import TerminalCloseDetail
from ..models.terminal_close_reason import TerminalCloseReason
from ..types import UNSET, Unset

T = TypeVar("T", bound="HostSessionEnded")


@_attrs_define
class HostSessionEnded:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        close_reason (TerminalCloseReason): Why a session ended (24, 10.3).
        close_detail (TerminalCloseDetail | Unset):
        exit_code (int | Unset):
    """

    session_id: UUID
    close_reason: TerminalCloseReason
    close_detail: TerminalCloseDetail | Unset = UNSET
    exit_code: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        close_reason = self.close_reason.value

        close_detail: str | Unset = UNSET
        if not isinstance(self.close_detail, Unset):
            close_detail = self.close_detail.value

        exit_code = self.exit_code

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "session_id": session_id,
                "close_reason": close_reason,
            }
        )
        if close_detail is not UNSET:
            field_dict["close_detail"] = close_detail
        if exit_code is not UNSET:
            field_dict["exit_code"] = exit_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        close_reason = TerminalCloseReason(d.pop("close_reason"))

        _close_detail = d.pop("close_detail", UNSET)
        close_detail: TerminalCloseDetail | Unset
        if isinstance(_close_detail, Unset):
            close_detail = UNSET
        else:
            close_detail = TerminalCloseDetail(_close_detail)

        exit_code = d.pop("exit_code", UNSET)

        host_session_ended = cls(
            session_id=session_id,
            close_reason=close_reason,
            close_detail=close_detail,
            exit_code=exit_code,
        )

        host_session_ended.additional_properties = d
        return host_session_ended

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
