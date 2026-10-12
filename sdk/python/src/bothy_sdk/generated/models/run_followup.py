from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_followup_state import RunFollowupState

T = TypeVar("T", bound="RunFollowup")


@_attrs_define
class RunFollowup:
    """
    Attributes:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        state (RunFollowupState):
    """

    message_id: UUID
    state: RunFollowupState
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_id = str(self.message_id)

        state = self.state.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_id": message_id,
                "state": state,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message_id = UUID(d.pop("message_id"))

        state = RunFollowupState(d.pop("state"))

        run_followup = cls(
            message_id=message_id,
            state=state,
        )

        run_followup.additional_properties = d
        return run_followup

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
