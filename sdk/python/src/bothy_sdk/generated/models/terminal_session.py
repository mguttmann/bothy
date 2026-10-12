from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.terminal_close_detail import TerminalCloseDetail
from ..models.terminal_close_reason import TerminalCloseReason
from ..models.terminal_session_state import TerminalSessionState
from ..types import UNSET, Unset

T = TypeVar("T", bound="TerminalSession")


@_attrs_define
class TerminalSession:
    """Metadata of a terminal session. Never content.

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        profile (str):
        state (TerminalSessionState):
        opened_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        bytes_in (int):
        bytes_out (int):
        version (int):
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        detached_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        closed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        close_reason (TerminalCloseReason | Unset): Why a session ended (24, 10.3).
        close_detail (TerminalCloseDetail | Unset):
        exit_code (int | Unset):
    """

    id: UUID
    host_id: UUID
    user_id: UUID
    device_id: UUID
    profile: str
    state: TerminalSessionState
    opened_at: datetime.datetime
    bytes_in: int
    bytes_out: int
    version: int
    started_at: datetime.datetime | Unset = UNSET
    detached_at: datetime.datetime | Unset = UNSET
    closed_at: datetime.datetime | Unset = UNSET
    close_reason: TerminalCloseReason | Unset = UNSET
    close_detail: TerminalCloseDetail | Unset = UNSET
    exit_code: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        host_id = str(self.host_id)

        user_id = str(self.user_id)

        device_id = str(self.device_id)

        profile = self.profile

        state = self.state.value

        opened_at = self.opened_at.isoformat()

        bytes_in = self.bytes_in

        bytes_out = self.bytes_out

        version = self.version

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        detached_at: str | Unset = UNSET
        if not isinstance(self.detached_at, Unset):
            detached_at = self.detached_at.isoformat()

        closed_at: str | Unset = UNSET
        if not isinstance(self.closed_at, Unset):
            closed_at = self.closed_at.isoformat()

        close_reason: str | Unset = UNSET
        if not isinstance(self.close_reason, Unset):
            close_reason = self.close_reason.value

        close_detail: str | Unset = UNSET
        if not isinstance(self.close_detail, Unset):
            close_detail = self.close_detail.value

        exit_code = self.exit_code

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "host_id": host_id,
                "user_id": user_id,
                "device_id": device_id,
                "profile": profile,
                "state": state,
                "opened_at": opened_at,
                "bytes_in": bytes_in,
                "bytes_out": bytes_out,
                "version": version,
            }
        )
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if detached_at is not UNSET:
            field_dict["detached_at"] = detached_at
        if closed_at is not UNSET:
            field_dict["closed_at"] = closed_at
        if close_reason is not UNSET:
            field_dict["close_reason"] = close_reason
        if close_detail is not UNSET:
            field_dict["close_detail"] = close_detail
        if exit_code is not UNSET:
            field_dict["exit_code"] = exit_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        host_id = UUID(d.pop("host_id"))

        user_id = UUID(d.pop("user_id"))

        device_id = UUID(d.pop("device_id"))

        profile = d.pop("profile")

        state = TerminalSessionState(d.pop("state"))

        opened_at = datetime.datetime.fromisoformat(d.pop("opened_at"))

        bytes_in = d.pop("bytes_in")

        bytes_out = d.pop("bytes_out")

        version = d.pop("version")

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _detached_at = d.pop("detached_at", UNSET)
        detached_at: datetime.datetime | Unset
        if isinstance(_detached_at, Unset):
            detached_at = UNSET
        else:
            detached_at = datetime.datetime.fromisoformat(_detached_at)

        _closed_at = d.pop("closed_at", UNSET)
        closed_at: datetime.datetime | Unset
        if isinstance(_closed_at, Unset):
            closed_at = UNSET
        else:
            closed_at = datetime.datetime.fromisoformat(_closed_at)

        _close_reason = d.pop("close_reason", UNSET)
        close_reason: TerminalCloseReason | Unset
        if isinstance(_close_reason, Unset):
            close_reason = UNSET
        else:
            close_reason = TerminalCloseReason(_close_reason)

        _close_detail = d.pop("close_detail", UNSET)
        close_detail: TerminalCloseDetail | Unset
        if isinstance(_close_detail, Unset):
            close_detail = UNSET
        else:
            close_detail = TerminalCloseDetail(_close_detail)

        exit_code = d.pop("exit_code", UNSET)

        terminal_session = cls(
            id=id,
            host_id=host_id,
            user_id=user_id,
            device_id=device_id,
            profile=profile,
            state=state,
            opened_at=opened_at,
            bytes_in=bytes_in,
            bytes_out=bytes_out,
            version=version,
            started_at=started_at,
            detached_at=detached_at,
            closed_at=closed_at,
            close_reason=close_reason,
            close_detail=close_detail,
            exit_code=exit_code,
        )

        terminal_session.additional_properties = d
        return terminal_session

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
