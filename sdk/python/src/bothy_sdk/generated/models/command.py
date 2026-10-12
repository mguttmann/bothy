from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.command_level import CommandLevel
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_option import CommandOption


T = TypeVar("T", bound="Command")


@_attrs_define
class Command:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        owner_principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        description (str):
        scope_kind (TargetKind): `host` is a connected host for the terminal; it has no level and only the capability
            `terminal` (24, 8.1).
        options (list[CommandOption]):
        required_level (CommandLevel):
        control (bool):
        version (int):
        scope_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    id: UUID
    owner_principal_id: UUID
    name: str
    description: str
    scope_kind: TargetKind
    options: list[CommandOption]
    required_level: CommandLevel
    control: bool
    version: int
    scope_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        owner_principal_id = str(self.owner_principal_id)

        name = self.name

        description = self.description

        scope_kind = self.scope_kind.value

        options = []
        for options_item_data in self.options:
            options_item = options_item_data.to_dict()
            options.append(options_item)

        required_level = self.required_level.value

        control = self.control

        version = self.version

        scope_id: str | Unset = UNSET
        if not isinstance(self.scope_id, Unset):
            scope_id = str(self.scope_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "owner_principal_id": owner_principal_id,
                "name": name,
                "description": description,
                "scope_kind": scope_kind,
                "options": options,
                "required_level": required_level,
                "control": control,
                "version": version,
            }
        )
        if scope_id is not UNSET:
            field_dict["scope_id"] = scope_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_option import CommandOption

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        owner_principal_id = UUID(d.pop("owner_principal_id"))

        name = d.pop("name")

        description = d.pop("description")

        scope_kind = TargetKind(d.pop("scope_kind"))

        options = []
        _options = d.pop("options")
        for options_item_data in _options:
            options_item = CommandOption.from_dict(options_item_data)

            options.append(options_item)

        required_level = CommandLevel(d.pop("required_level"))

        control = d.pop("control")

        version = d.pop("version")

        _scope_id = d.pop("scope_id", UNSET)
        scope_id: UUID | Unset
        if isinstance(_scope_id, Unset):
            scope_id = UNSET
        else:
            scope_id = UUID(_scope_id)

        command = cls(
            id=id,
            owner_principal_id=owner_principal_id,
            name=name,
            description=description,
            scope_kind=scope_kind,
            options=options,
            required_level=required_level,
            control=control,
            version=version,
            scope_id=scope_id,
        )

        command.additional_properties = d
        return command

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
