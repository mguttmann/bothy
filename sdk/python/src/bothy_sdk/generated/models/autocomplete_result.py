from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.command_choice import CommandChoice


T = TypeVar("T", bound="AutocompleteResult")


@_attrs_define
class AutocompleteResult:
    """
    Attributes:
        choices (list[CommandChoice]): At most `command.autocomplete_max_choices`.
    """

    choices: list[CommandChoice]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        choices = []
        for choices_item_data in self.choices:
            choices_item = choices_item_data.to_dict()
            choices.append(choices_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
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

        autocomplete_result = cls(
            choices=choices,
        )

        autocomplete_result.additional_properties = d
        return autocomplete_result

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
