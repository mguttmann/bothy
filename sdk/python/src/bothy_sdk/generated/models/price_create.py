from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.price_unit import PriceUnit
from ..types import UNSET, Unset

T = TypeVar("T", bound="PriceCreate")


@_attrs_define
class PriceCreate:
    """A price per unit (per million tokens for the `*_mtok` units) with its public source. `valid_to`, when present, lies
    after `valid_from`; `valid_from` may lie in the past. `source_url` is an absolute https URL without user info and
    fragment; `retrieved_at` is not in the future.

        Attributes:
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

    provider: str
    model: str
    unit: PriceUnit
    price_micros: int
    currency: str
    valid_from: datetime.datetime
    source_url: str
    retrieved_at: datetime.datetime
    valid_to: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
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

        field_dict.update(
            {
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

        price_create = cls(
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

        return price_create
