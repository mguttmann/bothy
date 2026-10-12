from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="Money")


@_attrs_define
class Money:
    """Amount in micro units of the currency. Never a float.

    Attributes:
        amount_micros (int):
        currency (str): ISO 4217 code.
        is_estimate (bool): True until the amount was reconciled with the provider invoice.
    """

    amount_micros: int
    currency: str
    is_estimate: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        amount_micros = self.amount_micros

        currency = self.currency

        is_estimate = self.is_estimate

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "amount_micros": amount_micros,
                "currency": currency,
                "is_estimate": is_estimate,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        amount_micros = d.pop("amount_micros")

        currency = d.pop("currency")

        is_estimate = d.pop("is_estimate")

        money = cls(
            amount_micros=amount_micros,
            currency=currency,
            is_estimate=is_estimate,
        )

        money.additional_properties = d
        return money

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
