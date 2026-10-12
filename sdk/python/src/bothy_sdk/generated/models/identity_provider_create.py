from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.identity_provider_kind import IdentityProviderKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="IdentityProviderCreate")


@_attrs_define
class IdentityProviderCreate:
    """
    Attributes:
        kind (IdentityProviderKind): Kind of identity provider (B9).
        name (str):
        config (JsonObject): Free-form JSON object whose schema is named in the field description.
        enabled (bool | Unset):
        allow_provisioning (bool | Unset):
    """

    kind: IdentityProviderKind
    name: str
    config: JsonObject
    enabled: bool | Unset = UNSET
    allow_provisioning: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        name = self.name

        config = self.config.to_dict()

        enabled = self.enabled

        allow_provisioning = self.allow_provisioning

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "name": name,
                "config": config,
            }
        )
        if enabled is not UNSET:
            field_dict["enabled"] = enabled
        if allow_provisioning is not UNSET:
            field_dict["allow_provisioning"] = allow_provisioning

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        kind = IdentityProviderKind(d.pop("kind"))

        name = d.pop("name")

        config = JsonObject.from_dict(d.pop("config"))

        enabled = d.pop("enabled", UNSET)

        allow_provisioning = d.pop("allow_provisioning", UNSET)

        identity_provider_create = cls(
            kind=kind,
            name=name,
            config=config,
            enabled=enabled,
            allow_provisioning=allow_provisioning,
        )

        return identity_provider_create
