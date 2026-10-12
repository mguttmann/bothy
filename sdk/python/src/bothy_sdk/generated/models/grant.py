from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.grant_subject_kind import GrantSubjectKind
from ..models.level import Level
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="Grant")


@_attrs_define
class Grant:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_kind (GrantSubjectKind):
        subject_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        target_kind (TargetKind): `host` is a connected host for the terminal; it has no level and only the capability
            `terminal` (24, 8.1).
        capabilities (list[Capability]):
        reason (str):
        granted_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        granted_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        level (Level | Unset): Grant level. Ordered `read` < `write` < `trigger`.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        revoked_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        revoked_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        revoke_reason (str | Unset):
    """

    id: UUID
    subject_kind: GrantSubjectKind
    subject_id: UUID
    target_kind: TargetKind
    capabilities: list[Capability]
    reason: str
    granted_by: UUID
    granted_at: datetime.datetime
    target_id: UUID | Unset = UNSET
    level: Level | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    revoked_at: datetime.datetime | Unset = UNSET
    revoked_by: UUID | Unset = UNSET
    revoke_reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        subject_kind = self.subject_kind.value

        subject_id = str(self.subject_id)

        target_kind = self.target_kind.value

        capabilities = []
        for capabilities_item_data in self.capabilities:
            capabilities_item = capabilities_item_data.value
            capabilities.append(capabilities_item)

        reason = self.reason

        granted_by = str(self.granted_by)

        granted_at = self.granted_at.isoformat()

        target_id: str | Unset = UNSET
        if not isinstance(self.target_id, Unset):
            target_id = str(self.target_id)

        level: str | Unset = UNSET
        if not isinstance(self.level, Unset):
            level = self.level.value

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        revoked_at: str | Unset = UNSET
        if not isinstance(self.revoked_at, Unset):
            revoked_at = self.revoked_at.isoformat()

        revoked_by: str | Unset = UNSET
        if not isinstance(self.revoked_by, Unset):
            revoked_by = str(self.revoked_by)

        revoke_reason = self.revoke_reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "subject_kind": subject_kind,
                "subject_id": subject_id,
                "target_kind": target_kind,
                "capabilities": capabilities,
                "reason": reason,
                "granted_by": granted_by,
                "granted_at": granted_at,
            }
        )
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
        if level is not UNSET:
            field_dict["level"] = level
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if revoked_at is not UNSET:
            field_dict["revoked_at"] = revoked_at
        if revoked_by is not UNSET:
            field_dict["revoked_by"] = revoked_by
        if revoke_reason is not UNSET:
            field_dict["revoke_reason"] = revoke_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        subject_kind = GrantSubjectKind(d.pop("subject_kind"))

        subject_id = UUID(d.pop("subject_id"))

        target_kind = TargetKind(d.pop("target_kind"))

        capabilities = []
        _capabilities = d.pop("capabilities")
        for capabilities_item_data in _capabilities:
            capabilities_item = Capability(capabilities_item_data)

            capabilities.append(capabilities_item)

        reason = d.pop("reason")

        granted_by = UUID(d.pop("granted_by"))

        granted_at = datetime.datetime.fromisoformat(d.pop("granted_at"))

        _target_id = d.pop("target_id", UNSET)
        target_id: UUID | Unset
        if isinstance(_target_id, Unset):
            target_id = UNSET
        else:
            target_id = UUID(_target_id)

        _level = d.pop("level", UNSET)
        level: Level | Unset
        if isinstance(_level, Unset):
            level = UNSET
        else:
            level = Level(_level)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        _revoked_at = d.pop("revoked_at", UNSET)
        revoked_at: datetime.datetime | Unset
        if isinstance(_revoked_at, Unset):
            revoked_at = UNSET
        else:
            revoked_at = datetime.datetime.fromisoformat(_revoked_at)

        _revoked_by = d.pop("revoked_by", UNSET)
        revoked_by: UUID | Unset
        if isinstance(_revoked_by, Unset):
            revoked_by = UNSET
        else:
            revoked_by = UUID(_revoked_by)

        revoke_reason = d.pop("revoke_reason", UNSET)

        grant = cls(
            id=id,
            subject_kind=subject_kind,
            subject_id=subject_id,
            target_kind=target_kind,
            capabilities=capabilities,
            reason=reason,
            granted_by=granted_by,
            granted_at=granted_at,
            target_id=target_id,
            level=level,
            expires_at=expires_at,
            revoked_at=revoked_at,
            revoked_by=revoked_by,
            revoke_reason=revoke_reason,
        )

        grant.additional_properties = d
        return grant

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
