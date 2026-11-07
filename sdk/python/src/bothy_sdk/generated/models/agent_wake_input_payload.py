from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.agent_wake_facts import AgentWakeFacts


T = TypeVar("T", bound="AgentWakeInputPayload")


@_attrs_define
class AgentWakeInputPayload:
    """Payload of `agent.wake_input` (04 15.9); the envelope carries a fresh `invocation_token` with `trigger_kind = wake`,
    `run_id`, `thread_id`, `wake_id` and `sub` per delivery.

        Attributes:
            run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            wake (AgentWakeFacts): The wake-up of a wake run or a wake input; the runner writes its label, reason and result
                as the input (09 5.8.3).
    """

    run_id: UUID
    thread_id: UUID
    wake_id: UUID
    wake: AgentWakeFacts
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        thread_id = str(self.thread_id)

        wake_id = str(self.wake_id)

        wake = self.wake.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "thread_id": thread_id,
                "wake_id": wake_id,
                "wake": wake,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_wake_facts import AgentWakeFacts

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        thread_id = UUID(d.pop("thread_id"))

        wake_id = UUID(d.pop("wake_id"))

        wake = AgentWakeFacts.from_dict(d.pop("wake"))

        agent_wake_input_payload = cls(
            run_id=run_id,
            thread_id=thread_id,
            wake_id=wake_id,
            wake=wake,
        )

        agent_wake_input_payload.additional_properties = d
        return agent_wake_input_payload

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
