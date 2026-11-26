from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.computer_close_detail import ComputerCloseDetail
from ..models.computer_close_reason import ComputerCloseReason
from ..models.computer_session_pause_reason import ComputerSessionPauseReason
from ..models.computer_session_state import ComputerSessionState
from ..types import UNSET, Unset

T = TypeVar("T", bound="ComputerSession")


@_attrs_define
class ComputerSession:
    """Metadata of a computer session (03 `computer_sessions`) with the name of its desktop. Never an image, a coordinate,
    typed text, a key name, the attach token or its `jti`.

        Attributes:
            id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            desktop (str): The name of the host (`hosts.name`).
            user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            state (ComputerSessionState): State of a computer session (docs/spec/27-computer-use.md 8.1).
            generation (int): The enablement generation the session was opened with (claim `gen`).
            key_generation (int): Generation of the host key the Noise handshake of the session used.
            opened_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            paused_ms (int): Time in milliseconds the session spent paused.
            action_count (int): Number of actions the session carried out.
            version (int): Version of the session row, raised by every change.
            pause_reason (ComputerSessionPauseReason | Unset): Why the session is paused; set only in state `paused`.
            attached_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            paused_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            closed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            stopped_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            close_reason (ComputerCloseReason | Unset): Why a computer session ended (docs/spec/27-computer-use.md 8.3).
            close_detail (ComputerCloseDetail | Unset): Closed list of 27 8.3: only with `stopped_by_user` (`local_escape`,
                `indicator_stop`, `client_stop`), `open_failed` (`token_invalid`, `token_expired`, `token_replayed`,
                `handshake_failed`, `helper_absent`, `helper_untrusted`, `not_enabled`, `busy`, `open_timeout`) and `error`
                (`stream_fault`, `helper_crashed`).
            front_app (str | Unset): Identity of the frontmost app of the last `computer.state` (bundle ID or executable
                name); never a window title.
    """

    id: UUID
    host_id: UUID
    desktop: str
    user_id: UUID
    agent_id: UUID
    run_id: UUID
    state: ComputerSessionState
    generation: int
    key_generation: int
    opened_at: datetime.datetime
    paused_ms: int
    action_count: int
    version: int
    pause_reason: ComputerSessionPauseReason | Unset = UNSET
    attached_at: datetime.datetime | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    paused_at: datetime.datetime | Unset = UNSET
    closed_at: datetime.datetime | Unset = UNSET
    stopped_by: UUID | Unset = UNSET
    close_reason: ComputerCloseReason | Unset = UNSET
    close_detail: ComputerCloseDetail | Unset = UNSET
    front_app: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        host_id = str(self.host_id)

        desktop = self.desktop

        user_id = str(self.user_id)

        agent_id = str(self.agent_id)

        run_id = str(self.run_id)

        state = self.state.value

        generation = self.generation

        key_generation = self.key_generation

        opened_at = self.opened_at.isoformat()

        paused_ms = self.paused_ms

        action_count = self.action_count

        version = self.version

        pause_reason: str | Unset = UNSET
        if not isinstance(self.pause_reason, Unset):
            pause_reason = self.pause_reason.value

        attached_at: str | Unset = UNSET
        if not isinstance(self.attached_at, Unset):
            attached_at = self.attached_at.isoformat()

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        paused_at: str | Unset = UNSET
        if not isinstance(self.paused_at, Unset):
            paused_at = self.paused_at.isoformat()

        closed_at: str | Unset = UNSET
        if not isinstance(self.closed_at, Unset):
            closed_at = self.closed_at.isoformat()

        stopped_by: str | Unset = UNSET
        if not isinstance(self.stopped_by, Unset):
            stopped_by = str(self.stopped_by)

        close_reason: str | Unset = UNSET
        if not isinstance(self.close_reason, Unset):
            close_reason = self.close_reason.value

        close_detail: str | Unset = UNSET
        if not isinstance(self.close_detail, Unset):
            close_detail = self.close_detail.value

        front_app = self.front_app

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "host_id": host_id,
                "desktop": desktop,
                "user_id": user_id,
                "agent_id": agent_id,
                "run_id": run_id,
                "state": state,
                "generation": generation,
                "key_generation": key_generation,
                "opened_at": opened_at,
                "paused_ms": paused_ms,
                "action_count": action_count,
                "version": version,
            }
        )
        if pause_reason is not UNSET:
            field_dict["pause_reason"] = pause_reason
        if attached_at is not UNSET:
            field_dict["attached_at"] = attached_at
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if paused_at is not UNSET:
            field_dict["paused_at"] = paused_at
        if closed_at is not UNSET:
            field_dict["closed_at"] = closed_at
        if stopped_by is not UNSET:
            field_dict["stopped_by"] = stopped_by
        if close_reason is not UNSET:
            field_dict["close_reason"] = close_reason
        if close_detail is not UNSET:
            field_dict["close_detail"] = close_detail
        if front_app is not UNSET:
            field_dict["front_app"] = front_app

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        host_id = UUID(d.pop("host_id"))

        desktop = d.pop("desktop")

        user_id = UUID(d.pop("user_id"))

        agent_id = UUID(d.pop("agent_id"))

        run_id = UUID(d.pop("run_id"))

        state = ComputerSessionState(d.pop("state"))

        generation = d.pop("generation")

        key_generation = d.pop("key_generation")

        opened_at = datetime.datetime.fromisoformat(d.pop("opened_at"))

        paused_ms = d.pop("paused_ms")

        action_count = d.pop("action_count")

        version = d.pop("version")

        _pause_reason = d.pop("pause_reason", UNSET)
        pause_reason: ComputerSessionPauseReason | Unset
        if isinstance(_pause_reason, Unset):
            pause_reason = UNSET
        else:
            pause_reason = ComputerSessionPauseReason(_pause_reason)

        _attached_at = d.pop("attached_at", UNSET)
        attached_at: datetime.datetime | Unset
        if isinstance(_attached_at, Unset):
            attached_at = UNSET
        else:
            attached_at = datetime.datetime.fromisoformat(_attached_at)

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _paused_at = d.pop("paused_at", UNSET)
        paused_at: datetime.datetime | Unset
        if isinstance(_paused_at, Unset):
            paused_at = UNSET
        else:
            paused_at = datetime.datetime.fromisoformat(_paused_at)

        _closed_at = d.pop("closed_at", UNSET)
        closed_at: datetime.datetime | Unset
        if isinstance(_closed_at, Unset):
            closed_at = UNSET
        else:
            closed_at = datetime.datetime.fromisoformat(_closed_at)

        _stopped_by = d.pop("stopped_by", UNSET)
        stopped_by: UUID | Unset
        if isinstance(_stopped_by, Unset):
            stopped_by = UNSET
        else:
            stopped_by = UUID(_stopped_by)

        _close_reason = d.pop("close_reason", UNSET)
        close_reason: ComputerCloseReason | Unset
        if isinstance(_close_reason, Unset):
            close_reason = UNSET
        else:
            close_reason = ComputerCloseReason(_close_reason)

        _close_detail = d.pop("close_detail", UNSET)
        close_detail: ComputerCloseDetail | Unset
        if isinstance(_close_detail, Unset):
            close_detail = UNSET
        else:
            close_detail = ComputerCloseDetail(_close_detail)

        front_app = d.pop("front_app", UNSET)

        computer_session = cls(
            id=id,
            host_id=host_id,
            desktop=desktop,
            user_id=user_id,
            agent_id=agent_id,
            run_id=run_id,
            state=state,
            generation=generation,
            key_generation=key_generation,
            opened_at=opened_at,
            paused_ms=paused_ms,
            action_count=action_count,
            version=version,
            pause_reason=pause_reason,
            attached_at=attached_at,
            started_at=started_at,
            paused_at=paused_at,
            closed_at=closed_at,
            stopped_by=stopped_by,
            close_reason=close_reason,
            close_detail=close_detail,
            front_app=front_app,
        )

        computer_session.additional_properties = d
        return computer_session

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
