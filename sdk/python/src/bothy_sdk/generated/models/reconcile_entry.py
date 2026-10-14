from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="ReconcileEntry")


@_attrs_define
class ReconcileEntry:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        amount_micros (int):
        currency (str):
    """

    run_id: UUID
    amount_micros: int
    currency: str

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        amount_micros = self.amount_micros

        currency = self.currency

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "run_id": run_id,
                "amount_micros": amount_micros,
                "currency": currency,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        amount_micros = d.pop("amount_micros")

        currency = d.pop("currency")

        reconcile_entry = cls(
            run_id=run_id,
            amount_micros=amount_micros,
            currency=currency,
        )

        return reconcile_entry
