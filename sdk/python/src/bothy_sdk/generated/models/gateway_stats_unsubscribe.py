from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="GatewayStatsUnsubscribe")


@_attrs_define
class GatewayStatsUnsubscribe:
    """End the `stats.live` subscription of the connection."""

    def to_dict(self) -> dict[str, Any]:

        field_dict: dict[str, Any] = {}

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        gateway_stats_unsubscribe = cls()

        return gateway_stats_unsubscribe
