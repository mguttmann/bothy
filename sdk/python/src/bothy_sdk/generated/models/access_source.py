from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.access_source_kind import AccessSourceKind
from ..models.access_source_rule import AccessSourceRule
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="AccessSource")


@_attrs_define
class AccessSource:
    """
    Attributes:
        kind (AccessSourceKind):
        grant_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        group_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        from_target_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no level and only the
            capability `terminal` (24, 8.1).
        from_target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        rule (AccessSourceRule | Unset): Protection rule of a `guard_blocked` source.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        membership_expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.
        reason (str | Unset): Reason of the grant.
    """

    kind: AccessSourceKind
    grant_id: UUID | Unset = UNSET
    group_id: UUID | Unset = UNSET
    from_target_kind: TargetKind | Unset = UNSET
    from_target_id: UUID | Unset = UNSET
    agent_id: UUID | Unset = UNSET
    rule: AccessSourceRule | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    membership_expires_at: datetime.datetime | Unset = UNSET
    reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        grant_id: str | Unset = UNSET
        if not isinstance(self.grant_id, Unset):
            grant_id = str(self.grant_id)

        group_id: str | Unset = UNSET
        if not isinstance(self.group_id, Unset):
            group_id = str(self.group_id)

        from_target_kind: str | Unset = UNSET
        if not isinstance(self.from_target_kind, Unset):
            from_target_kind = self.from_target_kind.value

        from_target_id: str | Unset = UNSET
        if not isinstance(self.from_target_id, Unset):
            from_target_id = str(self.from_target_id)

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        rule: str | Unset = UNSET
        if not isinstance(self.rule, Unset):
            rule = self.rule.value

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        membership_expires_at: str | Unset = UNSET
        if not isinstance(self.membership_expires_at, Unset):
            membership_expires_at = self.membership_expires_at.isoformat()

        reason = self.reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
            }
        )
        if grant_id is not UNSET:
            field_dict["grant_id"] = grant_id
        if group_id is not UNSET:
            field_dict["group_id"] = group_id
        if from_target_kind is not UNSET:
            field_dict["from_target_kind"] = from_target_kind
        if from_target_id is not UNSET:
            field_dict["from_target_id"] = from_target_id
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if rule is not UNSET:
            field_dict["rule"] = rule
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if membership_expires_at is not UNSET:
            field_dict["membership_expires_at"] = membership_expires_at
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = AccessSourceKind(d.pop("kind"))

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

        _from_target_kind = d.pop("from_target_kind", UNSET)
        from_target_kind: TargetKind | Unset
        if isinstance(_from_target_kind, Unset):
            from_target_kind = UNSET
        else:
            from_target_kind = TargetKind(_from_target_kind)

        _from_target_id = d.pop("from_target_id", UNSET)
        from_target_id: UUID | Unset
        if isinstance(_from_target_id, Unset):
            from_target_id = UNSET
        else:
            from_target_id = UUID(_from_target_id)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _rule = d.pop("rule", UNSET)
        rule: AccessSourceRule | Unset
        if isinstance(_rule, Unset):
            rule = UNSET
        else:
            rule = AccessSourceRule(_rule)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        _membership_expires_at = d.pop("membership_expires_at", UNSET)
        membership_expires_at: datetime.datetime | Unset
        if isinstance(_membership_expires_at, Unset):
            membership_expires_at = UNSET
        else:
            membership_expires_at = datetime.datetime.fromisoformat(
                _membership_expires_at
            )

        reason = d.pop("reason", UNSET)

        access_source = cls(
            kind=kind,
            grant_id=grant_id,
            group_id=group_id,
            from_target_kind=from_target_kind,
            from_target_id=from_target_id,
            agent_id=agent_id,
            rule=rule,
            expires_at=expires_at,
            membership_expires_at=membership_expires_at,
            reason=reason,
        )

        access_source.additional_properties = d
        return access_source

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
