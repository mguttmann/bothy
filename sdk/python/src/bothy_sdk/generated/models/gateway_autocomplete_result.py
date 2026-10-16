from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_choice import CommandChoice


T = TypeVar("T", bound="GatewayAutocompleteResult")


@_attrs_define
class GatewayAutocompleteResult:
    """Answer of the command owner to `command.autocomplete_requested` (action `command.autocomplete.respond`).

    Attributes:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        choices (list[CommandChoice]): At most `command.autocomplete_max_choices`.
        ref (str | Unset): Client reference echoed in `error` frames for this op.
    """

    request_id: UUID
    choices: list[CommandChoice]
    ref: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        request_id = str(self.request_id)

        choices = []
        for choices_item_data in self.choices:
            choices_item = choices_item_data.to_dict()
            choices.append(choices_item)

        ref = self.ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "request_id": request_id,
                "choices": choices,
            }
        )
        if ref is not UNSET:
            field_dict["ref"] = ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_choice import CommandChoice

        d = dict(src_dict)
        request_id = UUID(d.pop("request_id"))

        choices = []
        _choices = d.pop("choices")
        for choices_item_data in _choices:
            choices_item = CommandChoice.from_dict(choices_item_data)

            choices.append(choices_item)

        ref = d.pop("ref", UNSET)

        gateway_autocomplete_result = cls(
            request_id=request_id,
            choices=choices,
            ref=ref,
        )

        return gateway_autocomplete_result
