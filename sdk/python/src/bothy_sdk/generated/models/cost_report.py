from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="CostReport")


@_attrs_define
class CostReport:
    """Cost of one model in this run as reported by the adapter; stored as estimate.

    Attributes:
        provider (str):
        model (str):
        currency (str):
        tokens_in (int | Unset):
        tokens_out (int | Unset):
        tokens_cache_read (int | Unset):
        tokens_cache_write (int | Unset): Cache writes with 5 minutes (or without a stated duration).
        tokens_cache_write_1h (int | Unset): Cache writes with 1 hour, priced with `cache_write_1h_mtok`.
        amount_micros (int | Unset): Absent lets the server compute from the price table.
    """

    provider: str
    model: str
    currency: str
    tokens_in: int | Unset = UNSET
    tokens_out: int | Unset = UNSET
    tokens_cache_read: int | Unset = UNSET
    tokens_cache_write: int | Unset = UNSET
    tokens_cache_write_1h: int | Unset = UNSET
    amount_micros: int | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        provider = self.provider

        model = self.model

        currency = self.currency

        tokens_in = self.tokens_in

        tokens_out = self.tokens_out

        tokens_cache_read = self.tokens_cache_read

        tokens_cache_write = self.tokens_cache_write

        tokens_cache_write_1h = self.tokens_cache_write_1h

        amount_micros = self.amount_micros

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "provider": provider,
                "model": model,
                "currency": currency,
            }
        )
        if tokens_in is not UNSET:
            field_dict["tokens_in"] = tokens_in
        if tokens_out is not UNSET:
            field_dict["tokens_out"] = tokens_out
        if tokens_cache_read is not UNSET:
            field_dict["tokens_cache_read"] = tokens_cache_read
        if tokens_cache_write is not UNSET:
            field_dict["tokens_cache_write"] = tokens_cache_write
        if tokens_cache_write_1h is not UNSET:
            field_dict["tokens_cache_write_1h"] = tokens_cache_write_1h
        if amount_micros is not UNSET:
            field_dict["amount_micros"] = amount_micros

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        provider = d.pop("provider")

        model = d.pop("model")

        currency = d.pop("currency")

        tokens_in = d.pop("tokens_in", UNSET)

        tokens_out = d.pop("tokens_out", UNSET)

        tokens_cache_read = d.pop("tokens_cache_read", UNSET)

        tokens_cache_write = d.pop("tokens_cache_write", UNSET)

        tokens_cache_write_1h = d.pop("tokens_cache_write_1h", UNSET)

        amount_micros = d.pop("amount_micros", UNSET)

        cost_report = cls(
            provider=provider,
            model=model,
            currency=currency,
            tokens_in=tokens_in,
            tokens_out=tokens_out,
            tokens_cache_read=tokens_cache_read,
            tokens_cache_write=tokens_cache_write,
            tokens_cache_write_1h=tokens_cache_write_1h,
            amount_micros=amount_micros,
        )

        return cost_report
