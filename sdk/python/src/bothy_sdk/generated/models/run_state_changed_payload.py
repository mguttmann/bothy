from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.trigger_state import TriggerState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_summary import RunSummary


T = TypeVar("T", bound="RunStateChangedPayload")


@_attrs_define
class RunStateChangedPayload:
    """
    Attributes:
        run (RunSummary): Summary for sidebars, "running now" and collapsed live boxes.
        triggering_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        trigger_state (TriggerState | Unset):
    """

    run: RunSummary
    triggering_message_id: UUID | Unset = UNSET
    trigger_state: TriggerState | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run = self.run.to_dict()

        triggering_message_id: str | Unset = UNSET
        if not isinstance(self.triggering_message_id, Unset):
            triggering_message_id = str(self.triggering_message_id)

        trigger_state: str | Unset = UNSET
        if not isinstance(self.trigger_state, Unset):
            trigger_state = self.trigger_state.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run": run,
            }
        )
        if triggering_message_id is not UNSET:
            field_dict["triggering_message_id"] = triggering_message_id
        if trigger_state is not UNSET:
            field_dict["trigger_state"] = trigger_state

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_summary import RunSummary

        d = dict(src_dict)
        run = RunSummary.from_dict(d.pop("run"))

        _triggering_message_id = d.pop("triggering_message_id", UNSET)
        triggering_message_id: UUID | Unset
        if isinstance(_triggering_message_id, Unset):
            triggering_message_id = UNSET
        else:
            triggering_message_id = UUID(_triggering_message_id)

        _trigger_state = d.pop("trigger_state", UNSET)
        trigger_state: TriggerState | Unset
        if isinstance(_trigger_state, Unset):
            trigger_state = UNSET
        else:
            trigger_state = TriggerState(_trigger_state)

        run_state_changed_payload = cls(
            run=run,
            triggering_message_id=triggering_message_id,
            trigger_state=trigger_state,
        )

        run_state_changed_payload.additional_properties = d
        return run_state_changed_payload

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
