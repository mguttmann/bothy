from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.identity_provider_kind import IdentityProviderKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="IdentityProvider")


@_attrs_define
class IdentityProvider:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (IdentityProviderKind): Kind of identity provider (B9).
        name (str):
        config (JsonObject): Free-form JSON object whose schema is named in the field description.
        enabled (bool):
        allow_provisioning (bool):
        updated_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    kind: IdentityProviderKind
    name: str
    config: JsonObject
    enabled: bool
    allow_provisioning: bool
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        name = self.name

        config = self.config.to_dict()

        enabled = self.enabled

        allow_provisioning = self.allow_provisioning

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "kind": kind,
                "name": name,
                "config": config,
                "enabled": enabled,
                "allow_provisioning": allow_provisioning,
            }
        )
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = IdentityProviderKind(d.pop("kind"))

        name = d.pop("name")

        config = JsonObject.from_dict(d.pop("config"))

        enabled = d.pop("enabled")

        allow_provisioning = d.pop("allow_provisioning")

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        identity_provider = cls(
            id=id,
            kind=kind,
            name=name,
            config=config,
            enabled=enabled,
            allow_provisioning=allow_provisioning,
            updated_at=updated_at,
        )

        identity_provider.additional_properties = d
        return identity_provider

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
