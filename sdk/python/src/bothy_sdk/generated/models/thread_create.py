from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.message_create import MessageCreate


T = TypeVar("T", bound="ThreadCreate")


@_attrs_define
class ThreadCreate:
    """Creates a forum post (with `title` and `first_message`) or a thread from a message of the
    main thread (with `parent_message_id`, optional `first_message`). `first_message.run_id` is
    never valid, because a new thread has no run: `400 validation_failed` with pointer
    `/first_message/run_id` and `errors[].code` `run_mismatch`.

        Attributes:
            title (str | Unset):
            parent_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            tag_ids (list[UUID] | Unset): Forum posts only; at most `thread.max_tags`. Restricted tags need `manage`, as
                with `setThreadTags`.
            first_message (MessageCreate | Unset):
    """

    title: str | Unset = UNSET
    parent_message_id: UUID | Unset = UNSET
    tag_ids: list[UUID] | Unset = UNSET
    first_message: MessageCreate | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        parent_message_id: str | Unset = UNSET
        if not isinstance(self.parent_message_id, Unset):
            parent_message_id = str(self.parent_message_id)

        tag_ids: list[str] | Unset = UNSET
        if not isinstance(self.tag_ids, Unset):
            tag_ids = []
            for tag_ids_item_data in self.tag_ids:
                tag_ids_item = str(tag_ids_item_data)
                tag_ids.append(tag_ids_item)

        first_message: dict[str, Any] | Unset = UNSET
        if not isinstance(self.first_message, Unset):
            first_message = self.first_message.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if title is not UNSET:
            field_dict["title"] = title
        if parent_message_id is not UNSET:
            field_dict["parent_message_id"] = parent_message_id
        if tag_ids is not UNSET:
            field_dict["tag_ids"] = tag_ids
        if first_message is not UNSET:
            field_dict["first_message"] = first_message

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.message_create import MessageCreate

        d = dict(src_dict)
        title = d.pop("title", UNSET)

        _parent_message_id = d.pop("parent_message_id", UNSET)
        parent_message_id: UUID | Unset
        if isinstance(_parent_message_id, Unset):
            parent_message_id = UNSET
        else:
            parent_message_id = UUID(_parent_message_id)

        _tag_ids = d.pop("tag_ids", UNSET)
        tag_ids: list[UUID] | Unset = UNSET
        if _tag_ids is not UNSET:
            tag_ids = []
            for tag_ids_item_data in _tag_ids:
                tag_ids_item = UUID(tag_ids_item_data)

                tag_ids.append(tag_ids_item)

        _first_message = d.pop("first_message", UNSET)
        first_message: MessageCreate | Unset
        if isinstance(_first_message, Unset):
            first_message = UNSET
        else:
            first_message = MessageCreate.from_dict(_first_message)

        thread_create = cls(
            title=title,
            parent_message_id=parent_message_id,
            tag_ids=tag_ids,
            first_message=first_message,
        )

        return thread_create
