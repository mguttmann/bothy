from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.command_level import CommandLevel
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_option import CommandOption


T = TypeVar("T", bound="CommandRegister")


@_attrs_define
class CommandRegister:
    """
    Attributes:
        name (str):
        description (str):
        scope_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no level and only the
            capability `terminal` (24, 8.1).
        scope_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        options (list[CommandOption] | Unset):
        required_level (CommandLevel | Unset):
        control (bool | Unset):
    """

    name: str
    description: str
    scope_kind: TargetKind | Unset = UNSET
    scope_id: UUID | Unset = UNSET
    options: list[CommandOption] | Unset = UNSET
    required_level: CommandLevel | Unset = UNSET
    control: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        scope_kind: str | Unset = UNSET
        if not isinstance(self.scope_kind, Unset):
            scope_kind = self.scope_kind.value

        scope_id: str | Unset = UNSET
        if not isinstance(self.scope_id, Unset):
            scope_id = str(self.scope_id)

        options: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.options, Unset):
            options = []
            for options_item_data in self.options:
                options_item = options_item_data.to_dict()
                options.append(options_item)

        required_level: str | Unset = UNSET
        if not isinstance(self.required_level, Unset):
            required_level = self.required_level.value

        control = self.control

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "description": description,
            }
        )
        if scope_kind is not UNSET:
            field_dict["scope_kind"] = scope_kind
        if scope_id is not UNSET:
            field_dict["scope_id"] = scope_id
        if options is not UNSET:
            field_dict["options"] = options
        if required_level is not UNSET:
            field_dict["required_level"] = required_level
        if control is not UNSET:
            field_dict["control"] = control

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_option import CommandOption

        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description")

        _scope_kind = d.pop("scope_kind", UNSET)
        scope_kind: TargetKind | Unset
        if isinstance(_scope_kind, Unset):
            scope_kind = UNSET
        else:
            scope_kind = TargetKind(_scope_kind)

        _scope_id = d.pop("scope_id", UNSET)
        scope_id: UUID | Unset
        if isinstance(_scope_id, Unset):
            scope_id = UNSET
        else:
            scope_id = UUID(_scope_id)

        _options = d.pop("options", UNSET)
        options: list[CommandOption] | Unset = UNSET
        if _options is not UNSET:
            options = []
            for options_item_data in _options:
                options_item = CommandOption.from_dict(options_item_data)

                options.append(options_item)

        _required_level = d.pop("required_level", UNSET)
        required_level: CommandLevel | Unset
        if isinstance(_required_level, Unset):
            required_level = UNSET
        else:
            required_level = CommandLevel(_required_level)

        control = d.pop("control", UNSET)

        command_register = cls(
            name=name,
            description=description,
            scope_kind=scope_kind,
            scope_id=scope_id,
            options=options,
            required_level=required_level,
            control=control,
        )

        return command_register
