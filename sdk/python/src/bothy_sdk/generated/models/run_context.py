from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_context_message import RunContextMessage


T = TypeVar("T", bound="RunContext")


@_attrs_define
class RunContext:
    """Context of an own active run, schema `run_context.v1` (09, 4.6). Built by the server because only `authz` knows how
    trustworthy each author is.

        Attributes:
            run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            trigger_message (RunContextMessage):
            thread_context (list[RunContextMessage]):
            context_floor_seq (int):
            channel_context (list[RunContextMessage] | Unset): Only on the first run of a session in a thread with
                `parent_message_id`.
            truncated_before_seq (int | Unset): Set when older messages are missing; load them with `thread.read`.
    """

    run_id: UUID
    trigger_message: RunContextMessage
    thread_context: list[RunContextMessage]
    context_floor_seq: int
    channel_context: list[RunContextMessage] | Unset = UNSET
    truncated_before_seq: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        trigger_message = self.trigger_message.to_dict()

        thread_context = []
        for thread_context_item_data in self.thread_context:
            thread_context_item = thread_context_item_data.to_dict()
            thread_context.append(thread_context_item)

        context_floor_seq = self.context_floor_seq

        channel_context: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.channel_context, Unset):
            channel_context = []
            for channel_context_item_data in self.channel_context:
                channel_context_item = channel_context_item_data.to_dict()
                channel_context.append(channel_context_item)

        truncated_before_seq = self.truncated_before_seq

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "trigger_message": trigger_message,
                "thread_context": thread_context,
                "context_floor_seq": context_floor_seq,
            }
        )
        if channel_context is not UNSET:
            field_dict["channel_context"] = channel_context
        if truncated_before_seq is not UNSET:
            field_dict["truncated_before_seq"] = truncated_before_seq

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_context_message import RunContextMessage

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        trigger_message = RunContextMessage.from_dict(d.pop("trigger_message"))

        thread_context = []
        _thread_context = d.pop("thread_context")
        for thread_context_item_data in _thread_context:
            thread_context_item = RunContextMessage.from_dict(thread_context_item_data)

            thread_context.append(thread_context_item)

        context_floor_seq = d.pop("context_floor_seq")

        _channel_context = d.pop("channel_context", UNSET)
        channel_context: list[RunContextMessage] | Unset = UNSET
        if _channel_context is not UNSET:
            channel_context = []
            for channel_context_item_data in _channel_context:
                channel_context_item = RunContextMessage.from_dict(
                    channel_context_item_data
                )

                channel_context.append(channel_context_item)

        truncated_before_seq = d.pop("truncated_before_seq", UNSET)

        run_context = cls(
            run_id=run_id,
            trigger_message=trigger_message,
            thread_context=thread_context,
            context_floor_seq=context_floor_seq,
            channel_context=channel_context,
            truncated_before_seq=truncated_before_seq,
        )

        run_context.additional_properties = d
        return run_context

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
