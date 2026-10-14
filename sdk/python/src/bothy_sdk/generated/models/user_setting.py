from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="UserSetting")


@_attrs_define
class UserSetting:
    """
    Attributes:
        key (str):
        value (Any): Any JSON value except null. The schema of the value is defined by its key.
        version (int):
        updated_by_device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        updated_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    key: str
    value: Any
    version: int
    updated_by_device_id: UUID | Unset = UNSET
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        key = self.key

        value = self.value

        version = self.version

        updated_by_device_id: str | Unset = UNSET
        if not isinstance(self.updated_by_device_id, Unset):
            updated_by_device_id = str(self.updated_by_device_id)

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "key": key,
                "value": value,
                "version": version,
            }
        )
        if updated_by_device_id is not UNSET:
            field_dict["updated_by_device_id"] = updated_by_device_id
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        key = d.pop("key")

        value = d.pop("value")

        version = d.pop("version")

        _updated_by_device_id = d.pop("updated_by_device_id", UNSET)
        updated_by_device_id: UUID | Unset
        if isinstance(_updated_by_device_id, Unset):
            updated_by_device_id = UNSET
        else:
            updated_by_device_id = UUID(_updated_by_device_id)

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        user_setting = cls(
            key=key,
            value=value,
            version=version,
            updated_by_device_id=updated_by_device_id,
            updated_at=updated_at,
        )

        user_setting.additional_properties = d
        return user_setting

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
