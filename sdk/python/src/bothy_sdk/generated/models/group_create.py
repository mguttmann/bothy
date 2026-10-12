from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.group_source import GroupSource
from ..types import UNSET, Unset

T = TypeVar("T", bound="GroupCreate")


@_attrs_define
class GroupCreate:
    """
    Attributes:
        name (str):
        description (str | Unset):
        source (GroupSource | Unset):
        provider_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        external_ref (str | Unset): Required with `source = oidc_claim`; the claim value.
    """

    name: str
    description: str | Unset = UNSET
    source: GroupSource | Unset = UNSET
    provider_id: UUID | Unset = UNSET
    external_ref: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        source: str | Unset = UNSET
        if not isinstance(self.source, Unset):
            source = self.source.value

        provider_id: str | Unset = UNSET
        if not isinstance(self.provider_id, Unset):
            provider_id = str(self.provider_id)

        external_ref = self.external_ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
            }
        )
        if description is not UNSET:
            field_dict["description"] = description
        if source is not UNSET:
            field_dict["source"] = source
        if provider_id is not UNSET:
            field_dict["provider_id"] = provider_id
        if external_ref is not UNSET:
            field_dict["external_ref"] = external_ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description", UNSET)

        _source = d.pop("source", UNSET)
        source: GroupSource | Unset
        if isinstance(_source, Unset):
            source = UNSET
        else:
            source = GroupSource(_source)

        _provider_id = d.pop("provider_id", UNSET)
        provider_id: UUID | Unset
        if isinstance(_provider_id, Unset):
            provider_id = UNSET
        else:
            provider_id = UUID(_provider_id)

        external_ref = d.pop("external_ref", UNSET)

        group_create = cls(
            name=name,
            description=description,
            source=source,
            provider_id=provider_id,
            external_ref=external_ref,
        )

        return group_create
