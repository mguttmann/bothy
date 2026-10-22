from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.operator_mode import OperatorMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.resolve_i1 import ResolveI1


T = TypeVar("T", bound="OperatorSet")


@_attrs_define
class OperatorSet:
    """Operator assignment. `inherit` is only valid for channels.

    Attributes:
        mode (OperatorMode): `inherit` takes the default operator of the space (channels only).
        reason (str):
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        resolve_i1 (ResolveI1 | Unset): Guided flow (05 rule 1.6, 10 section 11): switch these agents to `organization`
            first, in the same transaction as the change.
    """

    mode: OperatorMode
    reason: str
    agent_id: UUID | Unset = UNSET
    resolve_i1: ResolveI1 | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        mode = self.mode.value

        reason = self.reason

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        resolve_i1: dict[str, Any] | Unset = UNSET
        if not isinstance(self.resolve_i1, Unset):
            resolve_i1 = self.resolve_i1.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "mode": mode,
                "reason": reason,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if resolve_i1 is not UNSET:
            field_dict["resolve_i1"] = resolve_i1

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.resolve_i1 import ResolveI1

        d = dict(src_dict)
        mode = OperatorMode(d.pop("mode"))

        reason = d.pop("reason")

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _resolve_i1 = d.pop("resolve_i1", UNSET)
        resolve_i1: ResolveI1 | Unset
        if isinstance(_resolve_i1, Unset):
            resolve_i1 = UNSET
        else:
            resolve_i1 = ResolveI1.from_dict(_resolve_i1)

        operator_set = cls(
            mode=mode,
            reason=reason,
            agent_id=agent_id,
            resolve_i1=resolve_i1,
        )

        return operator_set
