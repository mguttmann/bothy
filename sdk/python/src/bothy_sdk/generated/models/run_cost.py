from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.money import Money
    from ..models.token_counts import TokenCounts


T = TypeVar("T", bound="RunCost")


@_attrs_define
class RunCost:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        tokens (TokenCounts):
        unpriced (bool): A valid entry of the run has neither a price nor an amount of the adapter ("Price missing").
        agent_session_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        costs (list[Money] | Unset): One amount per currency; absent when amounts are hidden (mode `personal` without
            `billing.show_personal_estimates`).
    """

    run_id: UUID
    billing_mode: BillingMode
    tokens: TokenCounts
    unpriced: bool
    agent_session_id: UUID | Unset = UNSET
    costs: list[Money] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        billing_mode = self.billing_mode.value

        tokens = self.tokens.to_dict()

        unpriced = self.unpriced

        agent_session_id: str | Unset = UNSET
        if not isinstance(self.agent_session_id, Unset):
            agent_session_id = str(self.agent_session_id)

        costs: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.costs, Unset):
            costs = []
            for costs_item_data in self.costs:
                costs_item = costs_item_data.to_dict()
                costs.append(costs_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "billing_mode": billing_mode,
                "tokens": tokens,
                "unpriced": unpriced,
            }
        )
        if agent_session_id is not UNSET:
            field_dict["agent_session_id"] = agent_session_id
        if costs is not UNSET:
            field_dict["costs"] = costs

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money
        from ..models.token_counts import TokenCounts

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        tokens = TokenCounts.from_dict(d.pop("tokens"))

        unpriced = d.pop("unpriced")

        _agent_session_id = d.pop("agent_session_id", UNSET)
        agent_session_id: UUID | Unset
        if isinstance(_agent_session_id, Unset):
            agent_session_id = UNSET
        else:
            agent_session_id = UUID(_agent_session_id)

        _costs = d.pop("costs", UNSET)
        costs: list[Money] | Unset = UNSET
        if _costs is not UNSET:
            costs = []
            for costs_item_data in _costs:
                costs_item = Money.from_dict(costs_item_data)

                costs.append(costs_item)

        run_cost = cls(
            run_id=run_id,
            billing_mode=billing_mode,
            tokens=tokens,
            unpriced=unpriced,
            agent_session_id=agent_session_id,
            costs=costs,
        )

        run_cost.additional_properties = d
        return run_cost

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
