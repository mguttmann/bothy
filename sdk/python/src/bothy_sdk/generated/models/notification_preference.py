from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.notification_level import NotificationLevel
from ..models.notification_preference_target_kind import (
    NotificationPreferenceTargetKind,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="NotificationPreference")


@_attrs_define
class NotificationPreference:
    """
    Attributes:
        target_kind (NotificationPreferenceTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        level (NotificationLevel):
        version (int):
        muted_until (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    target_kind: NotificationPreferenceTargetKind
    target_id: UUID
    level: NotificationLevel
    version: int
    muted_until: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        target_kind = self.target_kind.value

        target_id = str(self.target_id)

        level = self.level.value

        version = self.version

        muted_until: str | Unset = UNSET
        if not isinstance(self.muted_until, Unset):
            muted_until = self.muted_until.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "target_kind": target_kind,
                "target_id": target_id,
                "level": level,
                "version": version,
            }
        )
        if muted_until is not UNSET:
            field_dict["muted_until"] = muted_until

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        target_kind = NotificationPreferenceTargetKind(d.pop("target_kind"))

        target_id = UUID(d.pop("target_id"))

        level = NotificationLevel(d.pop("level"))

        version = d.pop("version")

        _muted_until = d.pop("muted_until", UNSET)
        muted_until: datetime.datetime | Unset
        if isinstance(_muted_until, Unset):
            muted_until = UNSET
        else:
            muted_until = datetime.datetime.fromisoformat(_muted_until)

        notification_preference = cls(
            target_kind=target_kind,
            target_id=target_id,
            level=level,
            version=version,
            muted_until=muted_until,
        )

        notification_preference.additional_properties = d
        return notification_preference

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
