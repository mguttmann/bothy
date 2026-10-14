from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.host_state_set_state import HostStateSetState

T = TypeVar("T", bound="HostStateSet")


@_attrs_define
class HostStateSet:
    """
    Attributes:
        state (HostStateSetState):
        reason (str):
    """

    state: HostStateSetState
    reason: str

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        state = HostStateSetState(d.pop("state"))

        reason = d.pop("reason")

        host_state_set = cls(
            state=state,
            reason=reason,
        )

        return host_state_set
