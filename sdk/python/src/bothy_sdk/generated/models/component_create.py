from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.capability import Capability
from ..models.component_restriction import ComponentRestriction
from ..models.component_type import ComponentType
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ComponentCreate")


@_attrs_define
class ComponentCreate:
    """
    Attributes:
        custom_id (str):
        type_ (ComponentType):
        definition (JsonObject): Free-form JSON object whose schema is named in the field description.
        single_use (bool | Unset):
        required_capability (Capability | Unset): Capability next to the level. `terminal` exists only on the target
            `host`.
        restricted_to (ComponentRestriction | Unset): `run_starter`: only the person who started the run, or an owner.
            `owner`: only owners.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    custom_id: str
    type_: ComponentType
    definition: JsonObject
    single_use: bool | Unset = UNSET
    required_capability: Capability | Unset = UNSET
    restricted_to: ComponentRestriction | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        custom_id = self.custom_id

        type_ = self.type_.value

        definition = self.definition.to_dict()

        single_use = self.single_use

        required_capability: str | Unset = UNSET
        if not isinstance(self.required_capability, Unset):
            required_capability = self.required_capability.value

        restricted_to: str | Unset = UNSET
        if not isinstance(self.restricted_to, Unset):
            restricted_to = self.restricted_to.value

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "custom_id": custom_id,
                "type": type_,
                "definition": definition,
            }
        )
        if single_use is not UNSET:
            field_dict["single_use"] = single_use
        if required_capability is not UNSET:
            field_dict["required_capability"] = required_capability
        if restricted_to is not UNSET:
            field_dict["restricted_to"] = restricted_to
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        custom_id = d.pop("custom_id")

        type_ = ComponentType(d.pop("type"))

        definition = JsonObject.from_dict(d.pop("definition"))

        single_use = d.pop("single_use", UNSET)

        _required_capability = d.pop("required_capability", UNSET)
        required_capability: Capability | Unset
        if isinstance(_required_capability, Unset):
            required_capability = UNSET
        else:
            required_capability = Capability(_required_capability)

        _restricted_to = d.pop("restricted_to", UNSET)
        restricted_to: ComponentRestriction | Unset
        if isinstance(_restricted_to, Unset):
            restricted_to = UNSET
        else:
            restricted_to = ComponentRestriction(_restricted_to)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        component_create = cls(
            custom_id=custom_id,
            type_=type_,
            definition=definition,
            single_use=single_use,
            required_capability=required_capability,
            restricted_to=restricted_to,
            expires_at=expires_at,
        )

        return component_create
