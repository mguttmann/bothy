from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="Passkey")


@_attrs_define
class Passkey:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        backup_eligible (bool):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        aaguid (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        last_used_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    name: str
    backup_eligible: bool
    created_at: datetime.datetime
    aaguid: UUID | Unset = UNSET
    last_used_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        backup_eligible = self.backup_eligible

        created_at = self.created_at.isoformat()

        aaguid: str | Unset = UNSET
        if not isinstance(self.aaguid, Unset):
            aaguid = str(self.aaguid)

        last_used_at: str | Unset = UNSET
        if not isinstance(self.last_used_at, Unset):
            last_used_at = self.last_used_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "backup_eligible": backup_eligible,
                "created_at": created_at,
            }
        )
        if aaguid is not UNSET:
            field_dict["aaguid"] = aaguid
        if last_used_at is not UNSET:
            field_dict["last_used_at"] = last_used_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        backup_eligible = d.pop("backup_eligible")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _aaguid = d.pop("aaguid", UNSET)
        aaguid: UUID | Unset
        if isinstance(_aaguid, Unset):
            aaguid = UNSET
        else:
            aaguid = UUID(_aaguid)

        _last_used_at = d.pop("last_used_at", UNSET)
        last_used_at: datetime.datetime | Unset
        if isinstance(_last_used_at, Unset):
            last_used_at = UNSET
        else:
            last_used_at = datetime.datetime.fromisoformat(_last_used_at)

        passkey = cls(
            id=id,
            name=name,
            backup_eligible=backup_eligible,
            created_at=created_at,
            aaguid=aaguid,
            last_used_at=last_used_at,
        )

        passkey.additional_properties = d
        return passkey

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
