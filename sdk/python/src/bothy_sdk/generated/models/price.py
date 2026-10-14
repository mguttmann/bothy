from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.price_unit import PriceUnit
from ..types import UNSET, Unset

T = TypeVar("T", bound="Price")


@_attrs_define
class Price:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        provider (str):
        model (str):
        unit (PriceUnit):
        price_micros (int):
        currency (str):
        valid_from (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        source_url (str):
        retrieved_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        valid_to (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    provider: str
    model: str
    unit: PriceUnit
    price_micros: int
    currency: str
    valid_from: datetime.datetime
    source_url: str
    retrieved_at: datetime.datetime
    valid_to: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        provider = self.provider

        model = self.model

        unit = self.unit.value

        price_micros = self.price_micros

        currency = self.currency

        valid_from = self.valid_from.isoformat()

        source_url = self.source_url

        retrieved_at = self.retrieved_at.isoformat()

        valid_to: str | Unset = UNSET
        if not isinstance(self.valid_to, Unset):
            valid_to = self.valid_to.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "provider": provider,
                "model": model,
                "unit": unit,
                "price_micros": price_micros,
                "currency": currency,
                "valid_from": valid_from,
                "source_url": source_url,
                "retrieved_at": retrieved_at,
            }
        )
        if valid_to is not UNSET:
            field_dict["valid_to"] = valid_to

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        provider = d.pop("provider")

        model = d.pop("model")

        unit = PriceUnit(d.pop("unit"))

        price_micros = d.pop("price_micros")

        currency = d.pop("currency")

        valid_from = datetime.datetime.fromisoformat(d.pop("valid_from"))

        source_url = d.pop("source_url")

        retrieved_at = datetime.datetime.fromisoformat(d.pop("retrieved_at"))

        _valid_to = d.pop("valid_to", UNSET)
        valid_to: datetime.datetime | Unset
        if isinstance(_valid_to, Unset):
            valid_to = UNSET
        else:
            valid_to = datetime.datetime.fromisoformat(_valid_to)

        price = cls(
            id=id,
            provider=provider,
            model=model,
            unit=unit,
            price_micros=price_micros,
            currency=currency,
            valid_from=valid_from,
            source_url=source_url,
            retrieved_at=retrieved_at,
            valid_to=valid_to,
        )

        price.additional_properties = d
        return price

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
