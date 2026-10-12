from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode

if TYPE_CHECKING:
    from ..models.money import Money


T = TypeVar("T", bound="RunCost")


@_attrs_define
class RunCost:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        cost (Money): Amount in micro units of the currency. Never a float.
    """

    run_id: UUID
    billing_mode: BillingMode
    cost: Money
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        billing_mode = self.billing_mode.value

        cost = self.cost.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "billing_mode": billing_mode,
                "cost": cost,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        billing_mode = BillingMode(d.pop("billing_mode"))

        cost = Money.from_dict(d.pop("cost"))

        run_cost = cls(
            run_id=run_id,
            billing_mode=billing_mode,
            cost=cost,
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
