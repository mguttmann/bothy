from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.command_option_type import CommandOptionType
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_choice import CommandChoice


T = TypeVar("T", bound="CommandOption")


@_attrs_define
class CommandOption:
    """
    Attributes:
        name (str):
        description (str):
        type_ (CommandOptionType):
        required (bool):
        choices (list[CommandChoice] | Unset):
        autocomplete (bool | Unset):
    """

    name: str
    description: str
    type_: CommandOptionType
    required: bool
    choices: list[CommandChoice] | Unset = UNSET
    autocomplete: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        type_ = self.type_.value

        required = self.required

        choices: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.choices, Unset):
            choices = []
            for choices_item_data in self.choices:
                choices_item = choices_item_data.to_dict()
                choices.append(choices_item)

        autocomplete = self.autocomplete

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "description": description,
                "type": type_,
                "required": required,
            }
        )
        if choices is not UNSET:
            field_dict["choices"] = choices
        if autocomplete is not UNSET:
            field_dict["autocomplete"] = autocomplete

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_choice import CommandChoice

        d = dict(src_dict)
        name = d.pop("name")

        description = d.pop("description")

        type_ = CommandOptionType(d.pop("type"))

        required = d.pop("required")

        _choices = d.pop("choices", UNSET)
        choices: list[CommandChoice] | Unset = UNSET
        if _choices is not UNSET:
            choices = []
            for choices_item_data in _choices:
                choices_item = CommandChoice.from_dict(choices_item_data)

                choices.append(choices_item)

        autocomplete = d.pop("autocomplete", UNSET)

        command_option = cls(
            name=name,
            description=description,
            type_=type_,
            required=required,
            choices=choices,
            autocomplete=autocomplete,
        )

        return command_option
