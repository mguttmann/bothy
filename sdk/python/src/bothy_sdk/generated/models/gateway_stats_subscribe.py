from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="GatewayStatsSubscribe")


@_attrs_define
class GatewayStatsSubscribe:
    """Subscribe to `stats.live`; without `space_id` the instance (owner). Replaces an earlier subscription of the
    connection.

        Attributes:
            space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    space_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if space_id is not UNSET:
            field_dict["space_id"] = space_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        gateway_stats_subscribe = cls(
            space_id=space_id,
        )

        gateway_stats_subscribe.additional_properties = d
        return gateway_stats_subscribe

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
