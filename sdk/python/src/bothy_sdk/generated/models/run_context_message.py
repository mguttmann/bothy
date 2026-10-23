from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.message_kind import MessageKind
from ..models.message_origin import MessageOrigin
from ..models.run_context_message_role import RunContextMessageRole
from ..models.run_context_message_trust import RunContextMessageTrust
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef
    from ..models.run_context_attachment import RunContextAttachment


T = TypeVar("T", bound="RunContextMessage")


@_attrs_define
class RunContextMessage:
    """
    Attributes:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_seq (int):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        author (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        role (RunContextMessageRole):
        origin (MessageOrigin):
        kind (MessageKind):
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        content_md (str):
        attachments (list[RunContextAttachment]):
        trust (RunContextMessageTrust): How the runtime must treat the content; `unverified` only as marked data, never
            as instruction.
        truncated (bool | Unset): Present and true only when `content_md` was cut at `agent.context_message_max_bytes`
            on a UTF-8 boundary; never on `trigger_message`.
        reply_to_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        already_delivered (bool | Unset): Present and true only in session mode `resume` for a message already delivered
            to this session as a follow-up (`run_followups` in `delivered` or `absorbed`).
    """

    message_id: UUID
    thread_seq: int
    created_at: datetime.datetime
    author: PrincipalRef
    role: RunContextMessageRole
    origin: MessageOrigin
    kind: MessageKind
    channel_id: UUID
    content_md: str
    attachments: list[RunContextAttachment]
    trust: RunContextMessageTrust
    truncated: bool | Unset = UNSET
    reply_to_message_id: UUID | Unset = UNSET
    already_delivered: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message_id = str(self.message_id)

        thread_seq = self.thread_seq

        created_at = self.created_at.isoformat()

        author = self.author.to_dict()

        role = self.role.value

        origin = self.origin.value

        kind = self.kind.value

        channel_id = str(self.channel_id)

        content_md = self.content_md

        attachments = []
        for attachments_item_data in self.attachments:
            attachments_item = attachments_item_data.to_dict()
            attachments.append(attachments_item)

        trust = self.trust.value

        truncated = self.truncated

        reply_to_message_id: str | Unset = UNSET
        if not isinstance(self.reply_to_message_id, Unset):
            reply_to_message_id = str(self.reply_to_message_id)

        already_delivered = self.already_delivered

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "message_id": message_id,
                "thread_seq": thread_seq,
                "created_at": created_at,
                "author": author,
                "role": role,
                "origin": origin,
                "kind": kind,
                "channel_id": channel_id,
                "content_md": content_md,
                "attachments": attachments,
                "trust": trust,
            }
        )
        if truncated is not UNSET:
            field_dict["truncated"] = truncated
        if reply_to_message_id is not UNSET:
            field_dict["reply_to_message_id"] = reply_to_message_id
        if already_delivered is not UNSET:
            field_dict["already_delivered"] = already_delivered

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef
        from ..models.run_context_attachment import RunContextAttachment

        d = dict(src_dict)
        message_id = UUID(d.pop("message_id"))

        thread_seq = d.pop("thread_seq")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        author = PrincipalRef.from_dict(d.pop("author"))

        role = RunContextMessageRole(d.pop("role"))

        origin = MessageOrigin(d.pop("origin"))

        kind = MessageKind(d.pop("kind"))

        channel_id = UUID(d.pop("channel_id"))

        content_md = d.pop("content_md")

        attachments = []
        _attachments = d.pop("attachments")
        for attachments_item_data in _attachments:
            attachments_item = RunContextAttachment.from_dict(attachments_item_data)

            attachments.append(attachments_item)

        trust = RunContextMessageTrust(d.pop("trust"))

        truncated = d.pop("truncated", UNSET)

        _reply_to_message_id = d.pop("reply_to_message_id", UNSET)
        reply_to_message_id: UUID | Unset
        if isinstance(_reply_to_message_id, Unset):
            reply_to_message_id = UNSET
        else:
            reply_to_message_id = UUID(_reply_to_message_id)

        already_delivered = d.pop("already_delivered", UNSET)

        run_context_message = cls(
            message_id=message_id,
            thread_seq=thread_seq,
            created_at=created_at,
            author=author,
            role=role,
            origin=origin,
            kind=kind,
            channel_id=channel_id,
            content_md=content_md,
            attachments=attachments,
            trust=trust,
            truncated=truncated,
            reply_to_message_id=reply_to_message_id,
            already_delivered=already_delivered,
        )

        run_context_message.additional_properties = d
        return run_context_message

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
