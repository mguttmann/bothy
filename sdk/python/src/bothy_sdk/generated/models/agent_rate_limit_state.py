from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.agent_rate_limit_state_state import AgentRateLimitStateState
from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentRateLimitState")


@_attrs_define
class AgentRateLimitState:
    """Rate-limit state of the agent runtime at its provider as reported by the adapter (`agent_heartbeat.v1`). Never
    quotas of an account, keys or subscription names.

        Attributes:
            state (AgentRateLimitStateState):
            resets_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    state: AgentRateLimitStateState
    resets_at: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        resets_at: str | Unset = UNSET
        if not isinstance(self.resets_at, Unset):
            resets_at = self.resets_at.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
            }
        )
        if resets_at is not UNSET:
            field_dict["resets_at"] = resets_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        state = AgentRateLimitStateState(d.pop("state"))

        _resets_at = d.pop("resets_at", UNSET)
        resets_at: datetime.datetime | Unset
        if isinstance(_resets_at, Unset):
            resets_at = UNSET
        else:
            resets_at = datetime.datetime.fromisoformat(_resets_at)

        agent_rate_limit_state = cls(
            state=state,
            resets_at=resets_at,
        )

        return agent_rate_limit_state
