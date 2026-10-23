from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode

T = TypeVar("T", bound="CostTotal")


@_attrs_define
class CostTotal:
    """
    Attributes:
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        amount_micros (int):
        currency (str): ISO 4217 code.
        is_estimate (bool): True while any summed entry is an estimate.
    """

    billing_mode: BillingMode
    amount_micros: int
    currency: str
    is_estimate: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        billing_mode = self.billing_mode.value

        amount_micros = self.amount_micros

        currency = self.currency

        is_estimate = self.is_estimate

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "billing_mode": billing_mode,
                "amount_micros": amount_micros,
                "currency": currency,
                "is_estimate": is_estimate,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        billing_mode = BillingMode(d.pop("billing_mode"))

        amount_micros = d.pop("amount_micros")

        currency = d.pop("currency")

        is_estimate = d.pop("is_estimate")

        cost_total = cls(
            billing_mode=billing_mode,
            amount_micros=amount_micros,
            currency=currency,
            is_estimate=is_estimate,
        )

        cost_total.additional_properties = d
        return cost_total

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
