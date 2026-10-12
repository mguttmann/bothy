from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_runtime_state import AgentRuntimeState
from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_rate_limit_state import AgentRateLimitState


T = TypeVar("T", bound="StatsLiveAgent")


@_attrs_define
class StatsLiveAgent:
    """
    Attributes:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        presence (AgentRuntimeState): State derived from heartbeats.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        reported_billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        rate_limit (AgentRateLimitState | Unset): Rate-limit state of the agent runtime at its provider as reported by
            the adapter (`agent_heartbeat.v1`). Never quotas of an account, keys or subscription names.
    """

    agent_id: UUID
    presence: AgentRuntimeState
    billing_mode: BillingMode
    reported_billing_mode: BillingMode | Unset = UNSET
    rate_limit: AgentRateLimitState | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        presence = self.presence.value

        billing_mode = self.billing_mode.value

        reported_billing_mode: str | Unset = UNSET
        if not isinstance(self.reported_billing_mode, Unset):
            reported_billing_mode = self.reported_billing_mode.value

        rate_limit: dict[str, Any] | Unset = UNSET
        if not isinstance(self.rate_limit, Unset):
            rate_limit = self.rate_limit.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_id": agent_id,
                "presence": presence,
                "billing_mode": billing_mode,
            }
        )
        if reported_billing_mode is not UNSET:
            field_dict["reported_billing_mode"] = reported_billing_mode
        if rate_limit is not UNSET:
            field_dict["rate_limit"] = rate_limit

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_rate_limit_state import AgentRateLimitState

        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        presence = AgentRuntimeState(d.pop("presence"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        _reported_billing_mode = d.pop("reported_billing_mode", UNSET)
        reported_billing_mode: BillingMode | Unset
        if isinstance(_reported_billing_mode, Unset):
            reported_billing_mode = UNSET
        else:
            reported_billing_mode = BillingMode(_reported_billing_mode)

        _rate_limit = d.pop("rate_limit", UNSET)
        rate_limit: AgentRateLimitState | Unset
        if isinstance(_rate_limit, Unset):
            rate_limit = UNSET
        else:
            rate_limit = AgentRateLimitState.from_dict(_rate_limit)

        stats_live_agent = cls(
            agent_id=agent_id,
            presence=presence,
            billing_mode=billing_mode,
            reported_billing_mode=reported_billing_mode,
            rate_limit=rate_limit,
        )

        stats_live_agent.additional_properties = d
        return stats_live_agent

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
