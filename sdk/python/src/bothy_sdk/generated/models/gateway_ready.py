from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef
    from ..models.snapshot import Snapshot


T = TypeVar("T", bound="GatewayReady")


@_attrs_define
class GatewayReady:
    """
    Attributes:
        gateway_session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        permission_version (int):
        head_seq (int):
        snapshot (Snapshot): State without thread contents, delivered by `ready` and `GET /sync/snapshot`.
        sync_epoch (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    gateway_session_id: UUID
    principal: PrincipalRef
    permission_version: int
    head_seq: int
    snapshot: Snapshot
    sync_epoch: UUID
    device_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        gateway_session_id = str(self.gateway_session_id)

        principal = self.principal.to_dict()

        permission_version = self.permission_version

        head_seq = self.head_seq

        snapshot = self.snapshot.to_dict()

        sync_epoch = str(self.sync_epoch)

        device_id: str | Unset = UNSET
        if not isinstance(self.device_id, Unset):
            device_id = str(self.device_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "gateway_session_id": gateway_session_id,
                "principal": principal,
                "permission_version": permission_version,
                "head_seq": head_seq,
                "snapshot": snapshot,
                "sync_epoch": sync_epoch,
            }
        )
        if device_id is not UNSET:
            field_dict["device_id"] = device_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef
        from ..models.snapshot import Snapshot

        d = dict(src_dict)
        gateway_session_id = UUID(d.pop("gateway_session_id"))

        principal = PrincipalRef.from_dict(d.pop("principal"))

        permission_version = d.pop("permission_version")

        head_seq = d.pop("head_seq")

        snapshot = Snapshot.from_dict(d.pop("snapshot"))

        sync_epoch = UUID(d.pop("sync_epoch"))

        _device_id = d.pop("device_id", UNSET)
        device_id: UUID | Unset
        if isinstance(_device_id, Unset):
            device_id = UNSET
        else:
            device_id = UUID(_device_id)

        gateway_ready = cls(
            gateway_session_id=gateway_session_id,
            principal=principal,
            permission_version=permission_version,
            head_seq=head_seq,
            snapshot=snapshot,
            sync_epoch=sync_epoch,
            device_id=device_id,
        )

        gateway_ready.additional_properties = d
        return gateway_ready

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
