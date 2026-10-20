from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.capability import Capability
from ..models.grant_subject_kind import GrantSubjectKind
from ..models.level import Level
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.resolve_i1 import ResolveI1


T = TypeVar("T", bound="GrantCreate")


@_attrs_define
class GrantCreate:
    """`level` is required for every target except `host`, where it must be absent and `capabilities` must be exactly
    `[terminal]` (07, 2.12).

        Attributes:
            subject_kind (GrantSubjectKind):
            subject_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            target_kind (TargetKind): `host` is a connected host for the terminal; it has no level and only the capability
                `terminal` (24, 8.1).
            reason (str):
            target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            level (Level | Unset): Grant level. Ordered `read` < `write` < `trigger`.
            capabilities (list[Capability] | Unset):
            expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            resolve_i1 (ResolveI1 | Unset): Guided flow (05 rule 1.6, 10 section 11): switch these agents to `organization`
                first, in the same transaction as the change.
    """

    subject_kind: GrantSubjectKind
    subject_id: UUID
    target_kind: TargetKind
    reason: str
    target_id: UUID | Unset = UNSET
    level: Level | Unset = UNSET
    capabilities: list[Capability] | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    resolve_i1: ResolveI1 | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        subject_kind = self.subject_kind.value

        subject_id = str(self.subject_id)

        target_kind = self.target_kind.value

        reason = self.reason

        target_id: str | Unset = UNSET
        if not isinstance(self.target_id, Unset):
            target_id = str(self.target_id)

        level: str | Unset = UNSET
        if not isinstance(self.level, Unset):
            level = self.level.value

        capabilities: list[str] | Unset = UNSET
        if not isinstance(self.capabilities, Unset):
            capabilities = []
            for capabilities_item_data in self.capabilities:
                capabilities_item = capabilities_item_data.value
                capabilities.append(capabilities_item)

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        resolve_i1: dict[str, Any] | Unset = UNSET
        if not isinstance(self.resolve_i1, Unset):
            resolve_i1 = self.resolve_i1.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "subject_kind": subject_kind,
                "subject_id": subject_id,
                "target_kind": target_kind,
                "reason": reason,
            }
        )
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
        if level is not UNSET:
            field_dict["level"] = level
        if capabilities is not UNSET:
            field_dict["capabilities"] = capabilities
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if resolve_i1 is not UNSET:
            field_dict["resolve_i1"] = resolve_i1

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.resolve_i1 import ResolveI1

        d = dict(src_dict)
        subject_kind = GrantSubjectKind(d.pop("subject_kind"))

        subject_id = UUID(d.pop("subject_id"))

        target_kind = TargetKind(d.pop("target_kind"))

        reason = d.pop("reason")

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

        _capabilities = d.pop("capabilities", UNSET)
        capabilities: list[Capability] | Unset = UNSET
        if _capabilities is not UNSET:
            capabilities = []
            for capabilities_item_data in _capabilities:
                capabilities_item = Capability(capabilities_item_data)

                capabilities.append(capabilities_item)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        _resolve_i1 = d.pop("resolve_i1", UNSET)
        resolve_i1: ResolveI1 | Unset
        if isinstance(_resolve_i1, Unset):
            resolve_i1 = UNSET
        else:
            resolve_i1 = ResolveI1.from_dict(_resolve_i1)

        grant_create = cls(
            subject_kind=subject_kind,
            subject_id=subject_id,
            target_kind=target_kind,
            reason=reason,
            target_id=target_id,
            level=level,
            capabilities=capabilities,
            expires_at=expires_at,
            resolve_i1=resolve_i1,
        )

        return grant_create
