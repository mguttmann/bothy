from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.message import Message
    from ..models.thread import Thread


T = TypeVar("T", bound="ThreadCreated")


@_attrs_define
class ThreadCreated:
    """
    Attributes:
        thread (Thread):
        first_message (Message | Unset):
    """

    thread: Thread
    first_message: Message | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        thread = self.thread.to_dict()

        first_message: dict[str, Any] | Unset = UNSET
        if not isinstance(self.first_message, Unset):
            first_message = self.first_message.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "thread": thread,
            }
        )
        if first_message is not UNSET:
            field_dict["first_message"] = first_message

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.message import Message
        from ..models.thread import Thread

        d = dict(src_dict)
        thread = Thread.from_dict(d.pop("thread"))

        _first_message = d.pop("first_message", UNSET)
        first_message: Message | Unset
        if isinstance(_first_message, Unset):
            first_message = UNSET
        else:
            first_message = Message.from_dict(_first_message)

        thread_created = cls(
            thread=thread,
            first_message=first_message,
        )

        thread_created.additional_properties = d
        return thread_created

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
