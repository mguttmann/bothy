from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.gateway_resync_required_reason import GatewayResyncRequiredReason

T = TypeVar("T", bound="GatewayResyncRequired")


@_attrs_define
class GatewayResyncRequired:
    """
    Attributes:
        reason (GatewayResyncRequiredReason):
        head_seq (int):
        sync_epoch (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    reason: GatewayResyncRequiredReason
    head_seq: int
    sync_epoch: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        reason = self.reason.value

        head_seq = self.head_seq

        sync_epoch = str(self.sync_epoch)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "reason": reason,
                "head_seq": head_seq,
                "sync_epoch": sync_epoch,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        reason = GatewayResyncRequiredReason(d.pop("reason"))

        head_seq = d.pop("head_seq")

        sync_epoch = UUID(d.pop("sync_epoch"))

        gateway_resync_required = cls(
            reason=reason,
            head_seq=head_seq,
            sync_epoch=sync_epoch,
        )

        gateway_resync_required.additional_properties = d
        return gateway_resync_required

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
