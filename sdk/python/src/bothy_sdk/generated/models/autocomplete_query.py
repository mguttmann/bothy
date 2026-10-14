from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_option_value import CommandOptionValue


T = TypeVar("T", bound="AutocompleteQuery")


@_attrs_define
class AutocompleteQuery:
    """
    Attributes:
        option (str):
        value (str):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        options (list[CommandOptionValue] | Unset): Values of the other options entered so far.
    """

    option: str
    value: str
    thread_id: UUID | Unset = UNSET
    options: list[CommandOptionValue] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        option = self.option

        value = self.value

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        options: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.options, Unset):
            options = []
            for options_item_data in self.options:
                options_item = options_item_data.to_dict()
                options.append(options_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "option": option,
                "value": value,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if options is not UNSET:
            field_dict["options"] = options

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_option_value import CommandOptionValue

        d = dict(src_dict)
        option = d.pop("option")

        value = d.pop("value")

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _options = d.pop("options", UNSET)
        options: list[CommandOptionValue] | Unset = UNSET
        if _options is not UNSET:
            options = []
            for options_item_data in _options:
                options_item = CommandOptionValue.from_dict(options_item_data)

                options.append(options_item)

        autocomplete_query = cls(
            option=option,
            value=value,
            thread_id=thread_id,
            options=options,
        )

        return autocomplete_query
