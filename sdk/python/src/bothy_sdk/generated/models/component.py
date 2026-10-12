from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.component_restriction import ComponentRestriction
from ..models.component_state import ComponentState
from ..models.component_type import ComponentType
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="Component")


@_attrs_define
class Component:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        custom_id (str):
        type_ (ComponentType):
        definition (JsonObject): Free-form JSON object whose schema is named in the field description.
        position (int):
        single_use (bool):
        restricted_to (ComponentRestriction): `run_starter`: only the person who started the run, or an owner. `owner`:
            only owners.
        state (ComponentState):
        version (int):
        required_capability (Capability | Unset): Capability next to the level. `terminal` exists only on the target
            `host`.
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        used_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        used_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    custom_id: str
    type_: ComponentType
    definition: JsonObject
    position: int
    single_use: bool
    restricted_to: ComponentRestriction
    state: ComponentState
    version: int
    required_capability: Capability | Unset = UNSET
    run_id: UUID | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    used_by: UUID | Unset = UNSET
    used_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        custom_id = self.custom_id

        type_ = self.type_.value

        definition = self.definition.to_dict()

        position = self.position

        single_use = self.single_use

        restricted_to = self.restricted_to.value

        state = self.state.value

        version = self.version

        required_capability: str | Unset = UNSET
        if not isinstance(self.required_capability, Unset):
            required_capability = self.required_capability.value

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        used_by: str | Unset = UNSET
        if not isinstance(self.used_by, Unset):
            used_by = str(self.used_by)

        used_at: str | Unset = UNSET
        if not isinstance(self.used_at, Unset):
            used_at = self.used_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "custom_id": custom_id,
                "type": type_,
                "definition": definition,
                "position": position,
                "single_use": single_use,
                "restricted_to": restricted_to,
                "state": state,
                "version": version,
            }
        )
        if required_capability is not UNSET:
            field_dict["required_capability"] = required_capability
        if run_id is not UNSET:
            field_dict["run_id"] = run_id
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if used_by is not UNSET:
            field_dict["used_by"] = used_by
        if used_at is not UNSET:
            field_dict["used_at"] = used_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        custom_id = d.pop("custom_id")

        type_ = ComponentType(d.pop("type"))

        definition = JsonObject.from_dict(d.pop("definition"))

        position = d.pop("position")

        single_use = d.pop("single_use")

        restricted_to = ComponentRestriction(d.pop("restricted_to"))

        state = ComponentState(d.pop("state"))

        version = d.pop("version")

        _required_capability = d.pop("required_capability", UNSET)
        required_capability: Capability | Unset
        if isinstance(_required_capability, Unset):
            required_capability = UNSET
        else:
            required_capability = Capability(_required_capability)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        _used_by = d.pop("used_by", UNSET)
        used_by: UUID | Unset
        if isinstance(_used_by, Unset):
            used_by = UNSET
        else:
            used_by = UUID(_used_by)

        _used_at = d.pop("used_at", UNSET)
        used_at: datetime.datetime | Unset
        if isinstance(_used_at, Unset):
            used_at = UNSET
        else:
            used_at = datetime.datetime.fromisoformat(_used_at)

        component = cls(
            id=id,
            custom_id=custom_id,
            type_=type_,
            definition=definition,
            position=position,
            single_use=single_use,
            restricted_to=restricted_to,
            state=state,
            version=version,
            required_capability=required_capability,
            run_id=run_id,
            expires_at=expires_at,
            used_by=used_by,
            used_at=used_at,
        )

        component.additional_properties = d
        return component

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
