from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.feature_flag_kind import FeatureFlagKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.flag_rollout import FlagRollout


T = TypeVar("T", bound="FeatureFlag")


@_attrs_define
class FeatureFlag:
    """Registration from the code plus the stored value (ADR 0064). A key without a stored row has its default and
    `version` 0; a stored key without registration is `orphaned` and off.

        Attributes:
            key (str):
            orphaned (bool):
            enabled (bool): Global value; the stored value or the registered default.
            rollout (FlagRollout): Principals and groups for which the flag has the opposite value of `enabled`.
            version (int):
            kind (FeatureFlagKind | Unset): Absent for orphaned keys.
            description (str | Unset): From the registration (English); absent for orphaned keys.
            default (bool | Unset): Registered default; absent for orphaned keys.
            remove_by (str | Unset): Server version by which a `release` flag is removed.
            module (str | Unset): Module that owns the flag.
            updated_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            updated_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    key: str
    orphaned: bool
    enabled: bool
    rollout: FlagRollout
    version: int
    kind: FeatureFlagKind | Unset = UNSET
    description: str | Unset = UNSET
    default: bool | Unset = UNSET
    remove_by: str | Unset = UNSET
    module: str | Unset = UNSET
    updated_by: UUID | Unset = UNSET
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        key = self.key

        orphaned = self.orphaned

        enabled = self.enabled

        rollout = self.rollout.to_dict()

        version = self.version

        kind: str | Unset = UNSET
        if not isinstance(self.kind, Unset):
            kind = self.kind.value

        description = self.description

        default = self.default

        remove_by = self.remove_by

        module = self.module

        updated_by: str | Unset = UNSET
        if not isinstance(self.updated_by, Unset):
            updated_by = str(self.updated_by)

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "key": key,
                "orphaned": orphaned,
                "enabled": enabled,
                "rollout": rollout,
                "version": version,
            }
        )
        if kind is not UNSET:
            field_dict["kind"] = kind
        if description is not UNSET:
            field_dict["description"] = description
        if default is not UNSET:
            field_dict["default"] = default
        if remove_by is not UNSET:
            field_dict["remove_by"] = remove_by
        if module is not UNSET:
            field_dict["module"] = module
        if updated_by is not UNSET:
            field_dict["updated_by"] = updated_by
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.flag_rollout import FlagRollout

        d = dict(src_dict)
        key = d.pop("key")

        orphaned = d.pop("orphaned")

        enabled = d.pop("enabled")

        rollout = FlagRollout.from_dict(d.pop("rollout"))

        version = d.pop("version")

        _kind = d.pop("kind", UNSET)
        kind: FeatureFlagKind | Unset
        if isinstance(_kind, Unset):
            kind = UNSET
        else:
            kind = FeatureFlagKind(_kind)

        description = d.pop("description", UNSET)

        default = d.pop("default", UNSET)

        remove_by = d.pop("remove_by", UNSET)

        module = d.pop("module", UNSET)

        _updated_by = d.pop("updated_by", UNSET)
        updated_by: UUID | Unset
        if isinstance(_updated_by, Unset):
            updated_by = UNSET
        else:
            updated_by = UUID(_updated_by)

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        feature_flag = cls(
            key=key,
            orphaned=orphaned,
            enabled=enabled,
            rollout=rollout,
            version=version,
            kind=kind,
            description=description,
            default=default,
            remove_by=remove_by,
            module=module,
            updated_by=updated_by,
            updated_at=updated_at,
        )

        feature_flag.additional_properties = d
        return feature_flag

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
