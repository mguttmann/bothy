from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.notification_kind import NotificationKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="NotificationCreatedPayload")


@_attrs_define
class NotificationCreatedPayload:
    """
    Attributes:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (NotificationKind):
        quiet (bool): Inside quiet hours; show without sound.
        count (int): 1, or the unread count of a bundled hint (13 4.5).
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        space_name (str | Unset):
        channel_name (str | Unset):
        thread_title (str | Unset):
        sender_display_name (str | Unset):
        excerpt (str | Unset): Plain text, at most `notify.preview_max_chars`; only with `notify.preview_mode = full`.
        attachment_count (int | Unset):
        image_url (str | Unset): Fetch ticket for the `small` variant of the first image.
    """

    nid: UUID
    kind: NotificationKind
    quiet: bool
    count: int
    thread_id: UUID | Unset = UNSET
    message_id: UUID | Unset = UNSET
    target_id: UUID | Unset = UNSET
    space_name: str | Unset = UNSET
    channel_name: str | Unset = UNSET
    thread_title: str | Unset = UNSET
    sender_display_name: str | Unset = UNSET
    excerpt: str | Unset = UNSET
    attachment_count: int | Unset = UNSET
    image_url: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        nid = str(self.nid)

        kind = self.kind.value

        quiet = self.quiet

        count = self.count

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        message_id: str | Unset = UNSET
        if not isinstance(self.message_id, Unset):
            message_id = str(self.message_id)

        target_id: str | Unset = UNSET
        if not isinstance(self.target_id, Unset):
            target_id = str(self.target_id)

        space_name = self.space_name

        channel_name = self.channel_name

        thread_title = self.thread_title

        sender_display_name = self.sender_display_name

        excerpt = self.excerpt

        attachment_count = self.attachment_count

        image_url = self.image_url

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "nid": nid,
                "kind": kind,
                "quiet": quiet,
                "count": count,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if message_id is not UNSET:
            field_dict["message_id"] = message_id
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
        if space_name is not UNSET:
            field_dict["space_name"] = space_name
        if channel_name is not UNSET:
            field_dict["channel_name"] = channel_name
        if thread_title is not UNSET:
            field_dict["thread_title"] = thread_title
        if sender_display_name is not UNSET:
            field_dict["sender_display_name"] = sender_display_name
        if excerpt is not UNSET:
            field_dict["excerpt"] = excerpt
        if attachment_count is not UNSET:
            field_dict["attachment_count"] = attachment_count
        if image_url is not UNSET:
            field_dict["image_url"] = image_url

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        nid = UUID(d.pop("nid"))

        kind = NotificationKind(d.pop("kind"))

        quiet = d.pop("quiet")

        count = d.pop("count")

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _message_id = d.pop("message_id", UNSET)
        message_id: UUID | Unset
        if isinstance(_message_id, Unset):
            message_id = UNSET
        else:
            message_id = UUID(_message_id)

        _target_id = d.pop("target_id", UNSET)
        target_id: UUID | Unset
        if isinstance(_target_id, Unset):
            target_id = UNSET
        else:
            target_id = UUID(_target_id)

        space_name = d.pop("space_name", UNSET)

        channel_name = d.pop("channel_name", UNSET)

        thread_title = d.pop("thread_title", UNSET)

        sender_display_name = d.pop("sender_display_name", UNSET)

        excerpt = d.pop("excerpt", UNSET)

        attachment_count = d.pop("attachment_count", UNSET)

        image_url = d.pop("image_url", UNSET)

        notification_created_payload = cls(
            nid=nid,
            kind=kind,
            quiet=quiet,
            count=count,
            thread_id=thread_id,
            message_id=message_id,
            target_id=target_id,
            space_name=space_name,
            channel_name=channel_name,
            thread_title=thread_title,
            sender_display_name=sender_display_name,
            excerpt=excerpt,
            attachment_count=attachment_count,
            image_url=image_url,
        )

        notification_created_payload.additional_properties = d
        return notification_created_payload

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
