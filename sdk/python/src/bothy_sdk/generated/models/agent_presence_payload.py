from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_runtime_state import AgentRuntimeState
from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentPresencePayload")


@_attrs_define
class AgentPresencePayload:
    """
    Attributes:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (AgentRuntimeState): State derived from heartbeats.
        reported_billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
    """

    agent_id: UUID
    state: AgentRuntimeState
    reported_billing_mode: BillingMode | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        state = self.state.value

        reported_billing_mode: str | Unset = UNSET
        if not isinstance(self.reported_billing_mode, Unset):
            reported_billing_mode = self.reported_billing_mode.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_id": agent_id,
                "state": state,
            }
        )
        if reported_billing_mode is not UNSET:
            field_dict["reported_billing_mode"] = reported_billing_mode

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        state = AgentRuntimeState(d.pop("state"))

        _reported_billing_mode = d.pop("reported_billing_mode", UNSET)
        reported_billing_mode: BillingMode | Unset
        if isinstance(_reported_billing_mode, Unset):
            reported_billing_mode = UNSET
        else:
            reported_billing_mode = BillingMode(_reported_billing_mode)

        agent_presence_payload = cls(
            agent_id=agent_id,
            state=state,
            reported_billing_mode=reported_billing_mode,
        )

        agent_presence_payload.additional_properties = d
        return agent_presence_payload

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
