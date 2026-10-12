from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.group_source import GroupSource
from ..types import UNSET, Unset

T = TypeVar("T", bound="Group")


@_attrs_define
class Group:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        description (str):
        source (GroupSource):
        member_count (int):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        provider_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        external_ref (str | Unset): Claim value of the provider; set exactly for `source = oidc_claim`.
    """

    id: UUID
    name: str
    description: str
    source: GroupSource
    member_count: int
    created_at: datetime.datetime
    provider_id: UUID | Unset = UNSET
    external_ref: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        description = self.description

        source = self.source.value

        member_count = self.member_count

        created_at = self.created_at.isoformat()

        provider_id: str | Unset = UNSET
        if not isinstance(self.provider_id, Unset):
            provider_id = str(self.provider_id)

        external_ref = self.external_ref

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "description": description,
                "source": source,
                "member_count": member_count,
                "created_at": created_at,
            }
        )
        if provider_id is not UNSET:
            field_dict["provider_id"] = provider_id
        if external_ref is not UNSET:
            field_dict["external_ref"] = external_ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        description = d.pop("description")

        source = GroupSource(d.pop("source"))

        member_count = d.pop("member_count")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _provider_id = d.pop("provider_id", UNSET)
        provider_id: UUID | Unset
        if isinstance(_provider_id, Unset):
            provider_id = UNSET
        else:
            provider_id = UUID(_provider_id)

        external_ref = d.pop("external_ref", UNSET)

        group = cls(
            id=id,
            name=name,
            description=description,
            source=source,
            member_count=member_count,
            created_at=created_at,
            provider_id=provider_id,
            external_ref=external_ref,
        )

        group.additional_properties = d
        return group

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
