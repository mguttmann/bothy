from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.access_expiring_payload_kind import AccessExpiringPayloadKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.access_target import AccessTarget


T = TypeVar("T", bound="AccessExpiringPayload")


@_attrs_define
class AccessExpiringPayload:
    """A grant or group membership ends within `authz.expiry_reminder_lead`.

    Attributes:
        kind (AccessExpiringPayloadKind):
        subject_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        grant_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        group_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        target (AccessTarget | Unset):
    """

    kind: AccessExpiringPayloadKind
    subject_id: UUID
    expires_at: datetime.datetime
    grant_id: UUID | Unset = UNSET
    group_id: UUID | Unset = UNSET
    target: AccessTarget | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        subject_id = str(self.subject_id)

        expires_at = self.expires_at.isoformat()

        grant_id: str | Unset = UNSET
        if not isinstance(self.grant_id, Unset):
            grant_id = str(self.grant_id)

        group_id: str | Unset = UNSET
        if not isinstance(self.group_id, Unset):
            group_id = str(self.group_id)

        target: dict[str, Any] | Unset = UNSET
        if not isinstance(self.target, Unset):
            target = self.target.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
                "subject_id": subject_id,
                "expires_at": expires_at,
            }
        )
        if grant_id is not UNSET:
            field_dict["grant_id"] = grant_id
        if group_id is not UNSET:
            field_dict["group_id"] = group_id
        if target is not UNSET:
            field_dict["target"] = target

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_target import AccessTarget

        d = dict(src_dict)
        kind = AccessExpiringPayloadKind(d.pop("kind"))

        subject_id = UUID(d.pop("subject_id"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _grant_id = d.pop("grant_id", UNSET)
        grant_id: UUID | Unset
        if isinstance(_grant_id, Unset):
            grant_id = UNSET
        else:
            grant_id = UUID(_grant_id)

        _group_id = d.pop("group_id", UNSET)
        group_id: UUID | Unset
        if isinstance(_group_id, Unset):
            group_id = UNSET
        else:
            group_id = UUID(_group_id)

        _target = d.pop("target", UNSET)
        target: AccessTarget | Unset
        if isinstance(_target, Unset):
            target = UNSET
        else:
            target = AccessTarget.from_dict(_target)

        access_expiring_payload = cls(
            kind=kind,
            subject_id=subject_id,
            expires_at=expires_at,
            grant_id=grant_id,
            group_id=group_id,
            target=target,
        )

        access_expiring_payload.additional_properties = d
        return access_expiring_payload

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
