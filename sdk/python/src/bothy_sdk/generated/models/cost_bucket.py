from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="CostBucket")


@_attrs_define
class CostBucket:
    """
    Attributes:
        start (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        amount_micros (int):
        is_estimate (bool):
        group_key (str | Unset):
    """

    start: datetime.datetime
    billing_mode: BillingMode
    amount_micros: int
    is_estimate: bool
    group_key: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        start = self.start.isoformat()

        billing_mode = self.billing_mode.value

        amount_micros = self.amount_micros

        is_estimate = self.is_estimate

        group_key = self.group_key

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "start": start,
                "billing_mode": billing_mode,
                "amount_micros": amount_micros,
                "is_estimate": is_estimate,
            }
        )
        if group_key is not UNSET:
            field_dict["group_key"] = group_key

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        start = datetime.datetime.fromisoformat(d.pop("start"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        amount_micros = d.pop("amount_micros")

        is_estimate = d.pop("is_estimate")

        group_key = d.pop("group_key", UNSET)

        cost_bucket = cls(
            start=start,
            billing_mode=billing_mode,
            amount_micros=amount_micros,
            is_estimate=is_estimate,
            group_key=group_key,
        )

        cost_bucket.additional_properties = d
        return cost_bucket

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
