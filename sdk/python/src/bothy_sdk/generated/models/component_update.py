from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.component_update_state import ComponentUpdateState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ComponentUpdate")


@_attrs_define
class ComponentUpdate:
    """
    Attributes:
        state (ComponentUpdateState | Unset):
        definition (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    state: ComponentUpdateState | Unset = UNSET
    definition: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        state: str | Unset = UNSET
        if not isinstance(self.state, Unset):
            state = self.state.value

        definition: dict[str, Any] | Unset = UNSET
        if not isinstance(self.definition, Unset):
            definition = self.definition.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if state is not UNSET:
            field_dict["state"] = state
        if definition is not UNSET:
            field_dict["definition"] = definition

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        _state = d.pop("state", UNSET)
        state: ComponentUpdateState | Unset
        if isinstance(_state, Unset):
            state = UNSET
        else:
            state = ComponentUpdateState(_state)

        _definition = d.pop("definition", UNSET)
        definition: JsonObject | Unset
        if isinstance(_definition, Unset):
            definition = UNSET
        else:
            definition = JsonObject.from_dict(_definition)

        component_update = cls(
            state=state,
            definition=definition,
        )

        return component_update
