from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.host_computer_state_reason import HostComputerStateReason
from ..models.host_computer_state_state import HostComputerStateState

if TYPE_CHECKING:
    from ..models.host_computer_app import HostComputerApp


T = TypeVar("T", bound="HostComputerState")


@_attrs_define
class HostComputerState:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (HostComputerStateState):
        reason (HostComputerStateReason): `locked` exactly with `paused`, empty otherwise.
        action_count (int):
        front_app (HostComputerApp): Identity of a frontmost app (27 12.3 and 13.1); empty strings before the helper
            named one. Never a window title or a path.
    """

    session_id: UUID
    state: HostComputerStateState
    reason: HostComputerStateReason
    action_count: int
    front_app: HostComputerApp

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        state = self.state.value

        reason = self.reason.value

        action_count = self.action_count

        front_app = self.front_app.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "session_id": session_id,
                "state": state,
                "reason": reason,
                "action_count": action_count,
                "front_app": front_app,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_computer_app import HostComputerApp

        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        state = HostComputerStateState(d.pop("state"))

        reason = HostComputerStateReason(d.pop("reason"))

        action_count = d.pop("action_count")

        front_app = HostComputerApp.from_dict(d.pop("front_app"))

        host_computer_state = cls(
            session_id=session_id,
            state=state,
            reason=reason,
            action_count=action_count,
            front_app=front_app,
        )

        return host_computer_state
