from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode

T = TypeVar("T", bound="StatsBackendCost")


@_attrs_define
class StatsBackendCost:
    """
    Attributes:
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        currency (str):
        amount_micros (int):
        is_estimate (bool):
    """

    billing_mode: BillingMode
    currency: str
    amount_micros: int
    is_estimate: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        billing_mode = self.billing_mode.value

        currency = self.currency

        amount_micros = self.amount_micros

        is_estimate = self.is_estimate

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "billing_mode": billing_mode,
                "currency": currency,
                "amount_micros": amount_micros,
                "is_estimate": is_estimate,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        billing_mode = BillingMode(d.pop("billing_mode"))

        currency = d.pop("currency")

        amount_micros = d.pop("amount_micros")

        is_estimate = d.pop("is_estimate")

        stats_backend_cost = cls(
            billing_mode=billing_mode,
            currency=currency,
            amount_micros=amount_micros,
            is_estimate=is_estimate,
        )

        stats_backend_cost.additional_properties = d
        return stats_backend_cost

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
