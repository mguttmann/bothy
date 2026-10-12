from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="PriceClose")


@_attrs_define
class PriceClose:
    """
    Attributes:
        valid_to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    valid_to: datetime.datetime

    def to_dict(self) -> dict[str, Any]:
        valid_to = self.valid_to.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "valid_to": valid_to,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        valid_to = datetime.datetime.fromisoformat(d.pop("valid_to"))

        price_close = cls(
            valid_to=valid_to,
        )

        return price_close
