from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="IdentityProviderUpdate")


@_attrs_define
class IdentityProviderUpdate:
    """
    Attributes:
        name (str | Unset):
        config (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
        enabled (bool | Unset):
        allow_provisioning (bool | Unset):
    """

    name: str | Unset = UNSET
    config: JsonObject | Unset = UNSET
    enabled: bool | Unset = UNSET
    allow_provisioning: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.config, Unset):
            config = self.config.to_dict()

        enabled = self.enabled

        allow_provisioning = self.allow_provisioning

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if config is not UNSET:
            field_dict["config"] = config
        if enabled is not UNSET:
            field_dict["enabled"] = enabled
        if allow_provisioning is not UNSET:
            field_dict["allow_provisioning"] = allow_provisioning

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        name = d.pop("name", UNSET)

        _config = d.pop("config", UNSET)
        config: JsonObject | Unset
        if isinstance(_config, Unset):
            config = UNSET
        else:
            config = JsonObject.from_dict(_config)

        enabled = d.pop("enabled", UNSET)

        allow_provisioning = d.pop("allow_provisioning", UNSET)

        identity_provider_update = cls(
            name=name,
            config=config,
            enabled=enabled,
            allow_provisioning=allow_provisioning,
        )

        return identity_provider_update
