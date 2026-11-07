from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_wake_state import RunWakeState
from ..types import UNSET, Unset

T = TypeVar("T", bound="RunWakeChangedPayload")


@_attrs_define
class RunWakeChangedPayload:
    """Payload of `run.wake_changed` (09 5.8.2); ids and codes only, never the label, the condition or the result. Clients
    reload the thread's wake-ups with `listThreadWakes`, a runner with `listPendingRunWakes`.

        Attributes:
            wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            triggered_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            state (RunWakeState): State of a wake-up (09 5.8.1); every state other than `pending` is an end, and every end
                other than `fired` posts a note in the thread (09 5.8.4).
            state_reason (str | Unset): Reason code of the change.
            note_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            note_thread_seq (int | Unset): The `thread_seq` of the note; set with `note_message_id`.
    """

    wake_id: UUID
    thread_id: UUID
    agent_id: UUID
    triggered_by: UUID
    state: RunWakeState
    state_reason: str | Unset = UNSET
    note_message_id: UUID | Unset = UNSET
    note_thread_seq: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        wake_id = str(self.wake_id)

        thread_id = str(self.thread_id)

        agent_id = str(self.agent_id)

        triggered_by = str(self.triggered_by)

        state = self.state.value

        state_reason = self.state_reason

        note_message_id: str | Unset = UNSET
        if not isinstance(self.note_message_id, Unset):
            note_message_id = str(self.note_message_id)

        note_thread_seq = self.note_thread_seq

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "wake_id": wake_id,
                "thread_id": thread_id,
                "agent_id": agent_id,
                "triggered_by": triggered_by,
                "state": state,
            }
        )
        if state_reason is not UNSET:
            field_dict["state_reason"] = state_reason
        if note_message_id is not UNSET:
            field_dict["note_message_id"] = note_message_id
        if note_thread_seq is not UNSET:
            field_dict["note_thread_seq"] = note_thread_seq

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        wake_id = UUID(d.pop("wake_id"))

        thread_id = UUID(d.pop("thread_id"))

        agent_id = UUID(d.pop("agent_id"))

        triggered_by = UUID(d.pop("triggered_by"))

        state = RunWakeState(d.pop("state"))

        state_reason = d.pop("state_reason", UNSET)

        _note_message_id = d.pop("note_message_id", UNSET)
        note_message_id: UUID | Unset
        if isinstance(_note_message_id, Unset):
            note_message_id = UNSET
        else:
            note_message_id = UUID(_note_message_id)

        note_thread_seq = d.pop("note_thread_seq", UNSET)

        run_wake_changed_payload = cls(
            wake_id=wake_id,
            thread_id=thread_id,
            agent_id=agent_id,
            triggered_by=triggered_by,
            state=state,
            state_reason=state_reason,
            note_message_id=note_message_id,
            note_thread_seq=note_thread_seq,
        )

        run_wake_changed_payload.additional_properties = d
        return run_wake_changed_payload

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
