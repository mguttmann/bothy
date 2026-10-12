from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ClientMetricBucketsItem")


@_attrs_define
class ClientMetricBucketsItem:
    """
    Attributes:
        le (float):
        count (int):
    """

    le: float
    count: int

    def to_dict(self) -> dict[str, Any]:
        le = self.le

        count = self.count

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "le": le,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        le = d.pop("le")

        count = d.pop("count")

        client_metric_buckets_item = cls(
            le=le,
            count=count,
        )

        return client_metric_buckets_item
