from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.interaction_respond_state import InteractionRespondState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.interaction_response import InteractionResponse


T = TypeVar("T", bound="InteractionRespond")


@_attrs_define
class InteractionRespond:
    """
    Attributes:
        state (InteractionRespondState):
        response (InteractionResponse | Unset):
    """

    state: InteractionRespondState
    response: InteractionResponse | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        response: dict[str, Any] | Unset = UNSET
        if not isinstance(self.response, Unset):
            response = self.response.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
            }
        )
        if response is not UNSET:
            field_dict["response"] = response

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.interaction_response import InteractionResponse

        d = dict(src_dict)
        state = InteractionRespondState(d.pop("state"))

        _response = d.pop("response", UNSET)
        response: InteractionResponse | Unset
        if isinstance(_response, Unset):
            response = UNSET
        else:
            response = InteractionResponse.from_dict(_response)

        interaction_respond = cls(
            state=state,
            response=response,
        )

        return interaction_respond
