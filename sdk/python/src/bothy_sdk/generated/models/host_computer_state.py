from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.host_computer_state_reason import HostComputerStateReason
from ..models.host_computer_state_state import HostComputerStateState
from ..types import UNSET, Unset

T = TypeVar("T", bound="HostComputerState")


@_attrs_define
class HostComputerState:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (HostComputerStateState):
        action_count (int):
        reason (HostComputerStateReason | Unset): Present exactly with `paused`.
        front_app (str | Unset): Identity of the frontmost app (bundle ID or executable name).
    """

    session_id: UUID
    state: HostComputerStateState
    action_count: int
    reason: HostComputerStateReason | Unset = UNSET
    front_app: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        state = self.state.value

        action_count = self.action_count

        reason: str | Unset = UNSET
        if not isinstance(self.reason, Unset):
            reason = self.reason.value

        front_app = self.front_app

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "session_id": session_id,
                "state": state,
                "action_count": action_count,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason
        if front_app is not UNSET:
            field_dict["front_app"] = front_app

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        state = HostComputerStateState(d.pop("state"))

        action_count = d.pop("action_count")

        _reason = d.pop("reason", UNSET)
        reason: HostComputerStateReason | Unset
        if isinstance(_reason, Unset):
            reason = UNSET
        else:
            reason = HostComputerStateReason(_reason)

        front_app = d.pop("front_app", UNSET)

        host_computer_state = cls(
            session_id=session_id,
            state=state,
            action_count=action_count,
            reason=reason,
            front_app=front_app,
        )

        return host_computer_state
