from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_rate_limit_state import AgentRateLimitState
    from ..models.stats_backend_cost import StatsBackendCost
    from ..models.stats_backend_denials import StatsBackendDenials


T = TypeVar("T", bound="StatsBackendAgent")


@_attrs_define
class StatsBackendAgent:
    """
    Attributes:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        billing_modes_supported (list[BillingMode]):
        switches (int): Billing mode switches in the range.
        invocations_denied (StatsBackendDenials):
        reported_billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        costs (list[StatsBackendCost] | Unset): Only with `cost.read.aggregate`.
        rate_limit (AgentRateLimitState | Unset): Rate-limit state of the agent runtime at its provider as reported by
            the adapter (`agent_heartbeat.v1`). Never quotas of an account, keys or subscription names.
    """

    agent_id: UUID
    billing_mode: BillingMode
    billing_modes_supported: list[BillingMode]
    switches: int
    invocations_denied: StatsBackendDenials
    reported_billing_mode: BillingMode | Unset = UNSET
    costs: list[StatsBackendCost] | Unset = UNSET
    rate_limit: AgentRateLimitState | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        billing_mode = self.billing_mode.value

        billing_modes_supported = []
        for billing_modes_supported_item_data in self.billing_modes_supported:
            billing_modes_supported_item = billing_modes_supported_item_data.value
            billing_modes_supported.append(billing_modes_supported_item)

        switches = self.switches

        invocations_denied = self.invocations_denied.to_dict()

        reported_billing_mode: str | Unset = UNSET
        if not isinstance(self.reported_billing_mode, Unset):
            reported_billing_mode = self.reported_billing_mode.value

        costs: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.costs, Unset):
            costs = []
            for costs_item_data in self.costs:
                costs_item = costs_item_data.to_dict()
                costs.append(costs_item)

        rate_limit: dict[str, Any] | Unset = UNSET
        if not isinstance(self.rate_limit, Unset):
            rate_limit = self.rate_limit.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_id": agent_id,
                "billing_mode": billing_mode,
                "billing_modes_supported": billing_modes_supported,
                "switches": switches,
                "invocations_denied": invocations_denied,
            }
        )
        if reported_billing_mode is not UNSET:
            field_dict["reported_billing_mode"] = reported_billing_mode
        if costs is not UNSET:
            field_dict["costs"] = costs
        if rate_limit is not UNSET:
            field_dict["rate_limit"] = rate_limit

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_rate_limit_state import AgentRateLimitState
        from ..models.stats_backend_cost import StatsBackendCost
        from ..models.stats_backend_denials import StatsBackendDenials

        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        billing_modes_supported = []
        _billing_modes_supported = d.pop("billing_modes_supported")
        for billing_modes_supported_item_data in _billing_modes_supported:
            billing_modes_supported_item = BillingMode(
                billing_modes_supported_item_data
            )

            billing_modes_supported.append(billing_modes_supported_item)

        switches = d.pop("switches")

        invocations_denied = StatsBackendDenials.from_dict(d.pop("invocations_denied"))

        _reported_billing_mode = d.pop("reported_billing_mode", UNSET)
        reported_billing_mode: BillingMode | Unset
        if isinstance(_reported_billing_mode, Unset):
            reported_billing_mode = UNSET
        else:
            reported_billing_mode = BillingMode(_reported_billing_mode)

        _costs = d.pop("costs", UNSET)
        costs: list[StatsBackendCost] | Unset = UNSET
        if _costs is not UNSET:
            costs = []
            for costs_item_data in _costs:
                costs_item = StatsBackendCost.from_dict(costs_item_data)

                costs.append(costs_item)

        _rate_limit = d.pop("rate_limit", UNSET)
        rate_limit: AgentRateLimitState | Unset
        if isinstance(_rate_limit, Unset):
            rate_limit = UNSET
        else:
            rate_limit = AgentRateLimitState.from_dict(_rate_limit)

        stats_backend_agent = cls(
            agent_id=agent_id,
            billing_mode=billing_mode,
            billing_modes_supported=billing_modes_supported,
            switches=switches,
            invocations_denied=invocations_denied,
            reported_billing_mode=reported_billing_mode,
            costs=costs,
            rate_limit=rate_limit,
        )

        stats_backend_agent.additional_properties = d
        return stats_backend_agent

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
