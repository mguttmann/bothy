from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.host_computer_started_state import HostComputerStartedState

T = TypeVar("T", bound="HostComputerStarted")


@_attrs_define
class HostComputerStarted:
    """The host service started a computer session (`computer.started`, 27 12.3).

    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (HostComputerStartedState): State at the start; `paused` while the screen is locked.
    """

    session_id: UUID
    state: HostComputerStartedState

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        state = self.state.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "session_id": session_id,
                "state": state,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        state = HostComputerStartedState(d.pop("state"))

        host_computer_started = cls(
            session_id=session_id,
            state=state,
        )

        return host_computer_started
