from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.message_kind import MessageKind
from ..models.message_origin import MessageOrigin
from ..models.message_system_code import MessageSystemCode
from ..models.trigger_state import TriggerState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.attachment import Attachment
    from ..models.component import Component
    from ..models.principal_ref import PrincipalRef
    from ..models.reaction_summary import ReactionSummary


T = TypeVar("T", bound="Message")


@_attrs_define
class Message:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_seq (int):
        author (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        client_msg_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        origin (MessageOrigin):
        kind (MessageKind):
        content_md (str): Markdown. In lists longer contents are cut at `message.list_content_max_bytes` with
            `content_truncated`.
        content_format (int): Version of the Markdown dialect; `1` is CommonMark 0.31.2 without raw HTML, with mention
            tokens (03 `messages`).
        silent (bool):
        attachments (list[Attachment]):
        mention_ids (list[UUID]): Principals resolved from the mention tokens of `content_md`, sorted. Clients render a
            token as a mention only for these.
        reactions (list[ReactionSummary]):
        pinned (bool):
        components (list[Component]):
        version (int):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        system_code (MessageSystemCode | Unset): Set exactly for `kind = system`; clients render the localized line from
            it and `author`. `content_md` is empty then.
        content_truncated (bool | Unset): Set when `content_md` was cut (events above `sync.event_payload_max_bytes`,
            lists above `message.list_content_max_bytes`); `getMessage` returns the full content.
        reply_to_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        trigger_state (TriggerState | Unset):
        trigger_state_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        trigger_state_reason (str | Unset): Reason when `trigger_state` is `failed` or `cancelled`, e.g.
            `billing_mode_personal_not_holder`, `run_active_other_agent`, `user_cancelled`.
        my_reactions (list[str] | Unset): Emoji the caller reacted with. Only in REST responses, never in events.
        edited_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        deleted_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    thread_id: UUID
    channel_id: UUID
    thread_seq: int
    author: PrincipalRef
    client_msg_id: UUID
    origin: MessageOrigin
    kind: MessageKind
    content_md: str
    content_format: int
    silent: bool
    attachments: list[Attachment]
    mention_ids: list[UUID]
    reactions: list[ReactionSummary]
    pinned: bool
    components: list[Component]
    version: int
    created_at: datetime.datetime
    system_code: MessageSystemCode | Unset = UNSET
    content_truncated: bool | Unset = UNSET
    reply_to_message_id: UUID | Unset = UNSET
    run_id: UUID | Unset = UNSET
    trigger_state: TriggerState | Unset = UNSET
    trigger_state_at: datetime.datetime | Unset = UNSET
    trigger_state_reason: str | Unset = UNSET
    my_reactions: list[str] | Unset = UNSET
    edited_at: datetime.datetime | Unset = UNSET
    deleted_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        thread_id = str(self.thread_id)

        channel_id = str(self.channel_id)

        thread_seq = self.thread_seq

        author = self.author.to_dict()

        client_msg_id = str(self.client_msg_id)

        origin = self.origin.value

        kind = self.kind.value

        content_md = self.content_md

        content_format = self.content_format

        silent = self.silent

        attachments = []
        for attachments_item_data in self.attachments:
            attachments_item = attachments_item_data.to_dict()
            attachments.append(attachments_item)

        mention_ids = []
        for mention_ids_item_data in self.mention_ids:
            mention_ids_item = str(mention_ids_item_data)
            mention_ids.append(mention_ids_item)

        reactions = []
        for reactions_item_data in self.reactions:
            reactions_item = reactions_item_data.to_dict()
            reactions.append(reactions_item)

        pinned = self.pinned

        components = []
        for components_item_data in self.components:
            components_item = components_item_data.to_dict()
            components.append(components_item)

        version = self.version

        created_at = self.created_at.isoformat()

        system_code: str | Unset = UNSET
        if not isinstance(self.system_code, Unset):
            system_code = self.system_code.value

        content_truncated = self.content_truncated

        reply_to_message_id: str | Unset = UNSET
        if not isinstance(self.reply_to_message_id, Unset):
            reply_to_message_id = str(self.reply_to_message_id)

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        trigger_state: str | Unset = UNSET
        if not isinstance(self.trigger_state, Unset):
            trigger_state = self.trigger_state.value

        trigger_state_at: str | Unset = UNSET
        if not isinstance(self.trigger_state_at, Unset):
            trigger_state_at = self.trigger_state_at.isoformat()

        trigger_state_reason = self.trigger_state_reason

        my_reactions: list[str] | Unset = UNSET
        if not isinstance(self.my_reactions, Unset):
            my_reactions = self.my_reactions

        edited_at: str | Unset = UNSET
        if not isinstance(self.edited_at, Unset):
            edited_at = self.edited_at.isoformat()

        deleted_at: str | Unset = UNSET
        if not isinstance(self.deleted_at, Unset):
            deleted_at = self.deleted_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "thread_id": thread_id,
                "channel_id": channel_id,
                "thread_seq": thread_seq,
                "author": author,
                "client_msg_id": client_msg_id,
                "origin": origin,
                "kind": kind,
                "content_md": content_md,
                "content_format": content_format,
                "silent": silent,
                "attachments": attachments,
                "mention_ids": mention_ids,
                "reactions": reactions,
                "pinned": pinned,
                "components": components,
                "version": version,
                "created_at": created_at,
            }
        )
        if system_code is not UNSET:
            field_dict["system_code"] = system_code
        if content_truncated is not UNSET:
            field_dict["content_truncated"] = content_truncated
        if reply_to_message_id is not UNSET:
            field_dict["reply_to_message_id"] = reply_to_message_id
        if run_id is not UNSET:
            field_dict["run_id"] = run_id
        if trigger_state is not UNSET:
            field_dict["trigger_state"] = trigger_state
        if trigger_state_at is not UNSET:
            field_dict["trigger_state_at"] = trigger_state_at
        if trigger_state_reason is not UNSET:
            field_dict["trigger_state_reason"] = trigger_state_reason
        if my_reactions is not UNSET:
            field_dict["my_reactions"] = my_reactions
        if edited_at is not UNSET:
            field_dict["edited_at"] = edited_at
        if deleted_at is not UNSET:
            field_dict["deleted_at"] = deleted_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.attachment import Attachment
        from ..models.component import Component
        from ..models.principal_ref import PrincipalRef
        from ..models.reaction_summary import ReactionSummary

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        thread_id = UUID(d.pop("thread_id"))

        channel_id = UUID(d.pop("channel_id"))

        thread_seq = d.pop("thread_seq")

        author = PrincipalRef.from_dict(d.pop("author"))

        client_msg_id = UUID(d.pop("client_msg_id"))

        origin = MessageOrigin(d.pop("origin"))

        kind = MessageKind(d.pop("kind"))

        content_md = d.pop("content_md")

        content_format = d.pop("content_format")

        silent = d.pop("silent")

        attachments = []
        _attachments = d.pop("attachments")
        for attachments_item_data in _attachments:
            attachments_item = Attachment.from_dict(attachments_item_data)

            attachments.append(attachments_item)

        mention_ids = []
        _mention_ids = d.pop("mention_ids")
        for mention_ids_item_data in _mention_ids:
            mention_ids_item = UUID(mention_ids_item_data)

            mention_ids.append(mention_ids_item)

        reactions = []
        _reactions = d.pop("reactions")
        for reactions_item_data in _reactions:
            reactions_item = ReactionSummary.from_dict(reactions_item_data)

            reactions.append(reactions_item)

        pinned = d.pop("pinned")

        components = []
        _components = d.pop("components")
        for components_item_data in _components:
            components_item = Component.from_dict(components_item_data)

            components.append(components_item)

        version = d.pop("version")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _system_code = d.pop("system_code", UNSET)
        system_code: MessageSystemCode | Unset
        if isinstance(_system_code, Unset):
            system_code = UNSET
        else:
            system_code = MessageSystemCode(_system_code)

        content_truncated = d.pop("content_truncated", UNSET)

        _reply_to_message_id = d.pop("reply_to_message_id", UNSET)
        reply_to_message_id: UUID | Unset
        if isinstance(_reply_to_message_id, Unset):
            reply_to_message_id = UNSET
        else:
            reply_to_message_id = UUID(_reply_to_message_id)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        _trigger_state = d.pop("trigger_state", UNSET)
        trigger_state: TriggerState | Unset
        if isinstance(_trigger_state, Unset):
            trigger_state = UNSET
        else:
            trigger_state = TriggerState(_trigger_state)

        _trigger_state_at = d.pop("trigger_state_at", UNSET)
        trigger_state_at: datetime.datetime | Unset
        if isinstance(_trigger_state_at, Unset):
            trigger_state_at = UNSET
        else:
            trigger_state_at = datetime.datetime.fromisoformat(_trigger_state_at)

        trigger_state_reason = d.pop("trigger_state_reason", UNSET)

        my_reactions = cast(list[str], d.pop("my_reactions", UNSET))

        _edited_at = d.pop("edited_at", UNSET)
        edited_at: datetime.datetime | Unset
        if isinstance(_edited_at, Unset):
            edited_at = UNSET
        else:
            edited_at = datetime.datetime.fromisoformat(_edited_at)

        _deleted_at = d.pop("deleted_at", UNSET)
        deleted_at: datetime.datetime | Unset
        if isinstance(_deleted_at, Unset):
            deleted_at = UNSET
        else:
            deleted_at = datetime.datetime.fromisoformat(_deleted_at)

        message = cls(
            id=id,
            thread_id=thread_id,
            channel_id=channel_id,
            thread_seq=thread_seq,
            author=author,
            client_msg_id=client_msg_id,
            origin=origin,
            kind=kind,
            content_md=content_md,
            content_format=content_format,
            silent=silent,
            attachments=attachments,
            mention_ids=mention_ids,
            reactions=reactions,
            pinned=pinned,
            components=components,
            version=version,
            created_at=created_at,
            system_code=system_code,
            content_truncated=content_truncated,
            reply_to_message_id=reply_to_message_id,
            run_id=run_id,
            trigger_state=trigger_state,
            trigger_state_at=trigger_state_at,
            trigger_state_reason=trigger_state_reason,
            my_reactions=my_reactions,
            edited_at=edited_at,
            deleted_at=deleted_at,
        )

        message.additional_properties = d
        return message

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
