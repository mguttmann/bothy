from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="CostEstimate")


@_attrs_define
class CostEstimate:
    """Estimate of one run in the requested range for the reconciliation (10, 12.4; 05, R-AZ-28). Names no channel, thread
    or content.

        Attributes:
            run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            provider (str):
            currency (str): ISO 4217 code.
            estimate_micros (int): Sum in micro units of the run's estimate entries in the range whose successor, if any, is
                not an estimate; the estimate before any reconciliation.
            reconciled (bool): True when every valid entry of the run in the range has `source = reconciled`.
    """

    run_id: UUID
    agent_id: UUID
    provider: str
    currency: str
    estimate_micros: int
    reconciled: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        agent_id = str(self.agent_id)

        provider = self.provider

        currency = self.currency

        estimate_micros = self.estimate_micros

        reconciled = self.reconciled

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "agent_id": agent_id,
                "provider": provider,
                "currency": currency,
                "estimate_micros": estimate_micros,
                "reconciled": reconciled,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        agent_id = UUID(d.pop("agent_id"))

        provider = d.pop("provider")

        currency = d.pop("currency")

        estimate_micros = d.pop("estimate_micros")

        reconciled = d.pop("reconciled")

        cost_estimate = cls(
            run_id=run_id,
            agent_id=agent_id,
            provider=provider,
            currency=currency,
            estimate_micros=estimate_micros,
            reconciled=reconciled,
        )

        cost_estimate.additional_properties = d
        return cost_estimate

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
