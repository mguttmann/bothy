from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.operator_mode import OperatorMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="OperatorSet")


@_attrs_define
class OperatorSet:
    """Operator assignment. `inherit` is only valid for channels.

    Attributes:
        mode (OperatorMode): `inherit` takes the default operator of the space (channels only).
        reason (str):
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    mode: OperatorMode
    reason: str
    agent_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        mode = self.mode.value

        reason = self.reason

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "mode": mode,
                "reason": reason,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        mode = OperatorMode(d.pop("mode"))

        reason = d.pop("reason")

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        operator_set = cls(
            mode=mode,
            reason=reason,
            agent_id=agent_id,
        )

        return operator_set
