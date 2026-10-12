from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.command_choice import CommandChoice


T = TypeVar("T", bound="AutocompleteAnswer")


@_attrs_define
class AutocompleteAnswer:
    """
    Attributes:
        choices (list[CommandChoice]):
    """

    choices: list[CommandChoice]

    def to_dict(self) -> dict[str, Any]:
        choices = []
        for choices_item_data in self.choices:
            choices_item = choices_item_data.to_dict()
            choices.append(choices_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "choices": choices,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_choice import CommandChoice

        d = dict(src_dict)
        choices = []
        _choices = d.pop("choices")
        for choices_item_data in _choices:
            choices_item = CommandChoice.from_dict(choices_item_data)

            choices.append(choices_item)

        autocomplete_answer = cls(
            choices=choices,
        )

        return autocomplete_answer
