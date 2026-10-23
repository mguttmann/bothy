from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.message_create_kind import MessageCreateKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.component_create import ComponentCreate


T = TypeVar("T", bound="MessageCreate")


@_attrs_define
class MessageCreate:
    """
    Attributes:
        client_msg_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (MessageCreateKind | Unset):
        content_md (str | Unset): Empty or white space only content is allowed only when `attachment_file_ids` lists at
            least one file (a picture without caption); otherwise `400 validation_failed` with `errors[].code` `empty`.
        reply_to_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        attachment_file_ids (list[UUID] | Unset): Own files uploaded for the channel of this message with a completed
            upload session, in the order they appear in the message; at most `message.max_attachments`. Needs the right
            `upload` in addition to `write` (`403 forbidden` with `reason` `capability_missing`). A file that is not
            attachable (not the caller's, another channel, upload not completed or failed, deleted, already attached, or
            listed twice) answers `409 conflict` with `reason` `file_not_attachable` and the pointer of the entry in
            `detail`; more than `message.max_attachments` answer `400 validation_failed` with `errors[].code`
            `limit_reached`. Files whose scan has not ended are attachable.
        mention_ids (list[UUID] | Unset): Optional check. The server resolves mentions from `content_md` (03 `messages`,
            content format; 07 section 4.6); when present, the set must equal the resolved set, else `400 validation_failed`
            with code `mention_mismatch`.
        silent (bool | Unset):
        clear_draft (bool | Unset):
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        components (list[ComponentCreate] | Unset): Requires `kind = card`, and `kind = card` requires at least one
            component; at most `component.max_per_message`. Agents and apps send cards (people get `invalid`). An app needs
            `write` and `messages:write` as for every message; without `components:write` its components stay inactive, and
            a use answers `409 component_used`. The whole `message.created` payload of the card must fit
            `sync.event_payload_max_bytes`, else `too_long` on `/components`. An agent whose owner switched
            `supports_components` off gets `403 feature_disabled` (09, 2.2 and 11.1); the switch does not apply to apps.
    """

    client_msg_id: UUID
    kind: MessageCreateKind | Unset = UNSET
    content_md: str | Unset = UNSET
    reply_to_message_id: UUID | Unset = UNSET
    attachment_file_ids: list[UUID] | Unset = UNSET
    mention_ids: list[UUID] | Unset = UNSET
    silent: bool | Unset = UNSET
    clear_draft: bool | Unset = UNSET
    run_id: UUID | Unset = UNSET
    components: list[ComponentCreate] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        client_msg_id = str(self.client_msg_id)

        kind: str | Unset = UNSET
        if not isinstance(self.kind, Unset):
            kind = self.kind.value

        content_md = self.content_md

        reply_to_message_id: str | Unset = UNSET
        if not isinstance(self.reply_to_message_id, Unset):
            reply_to_message_id = str(self.reply_to_message_id)

        attachment_file_ids: list[str] | Unset = UNSET
        if not isinstance(self.attachment_file_ids, Unset):
            attachment_file_ids = []
            for attachment_file_ids_item_data in self.attachment_file_ids:
                attachment_file_ids_item = str(attachment_file_ids_item_data)
                attachment_file_ids.append(attachment_file_ids_item)

        mention_ids: list[str] | Unset = UNSET
        if not isinstance(self.mention_ids, Unset):
            mention_ids = []
            for mention_ids_item_data in self.mention_ids:
                mention_ids_item = str(mention_ids_item_data)
                mention_ids.append(mention_ids_item)

        silent = self.silent

        clear_draft = self.clear_draft

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        components: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.components, Unset):
            components = []
            for components_item_data in self.components:
                components_item = components_item_data.to_dict()
                components.append(components_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "client_msg_id": client_msg_id,
            }
        )
        if kind is not UNSET:
            field_dict["kind"] = kind
        if content_md is not UNSET:
            field_dict["content_md"] = content_md
        if reply_to_message_id is not UNSET:
            field_dict["reply_to_message_id"] = reply_to_message_id
        if attachment_file_ids is not UNSET:
            field_dict["attachment_file_ids"] = attachment_file_ids
        if mention_ids is not UNSET:
            field_dict["mention_ids"] = mention_ids
        if silent is not UNSET:
            field_dict["silent"] = silent
        if clear_draft is not UNSET:
            field_dict["clear_draft"] = clear_draft
        if run_id is not UNSET:
            field_dict["run_id"] = run_id
        if components is not UNSET:
            field_dict["components"] = components

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.component_create import ComponentCreate

        d = dict(src_dict)
        client_msg_id = UUID(d.pop("client_msg_id"))

        _kind = d.pop("kind", UNSET)
        kind: MessageCreateKind | Unset
        if isinstance(_kind, Unset):
            kind = UNSET
        else:
            kind = MessageCreateKind(_kind)

        content_md = d.pop("content_md", UNSET)

        _reply_to_message_id = d.pop("reply_to_message_id", UNSET)
        reply_to_message_id: UUID | Unset
        if isinstance(_reply_to_message_id, Unset):
            reply_to_message_id = UNSET
        else:
            reply_to_message_id = UUID(_reply_to_message_id)

        _attachment_file_ids = d.pop("attachment_file_ids", UNSET)
        attachment_file_ids: list[UUID] | Unset = UNSET
        if _attachment_file_ids is not UNSET:
            attachment_file_ids = []
            for attachment_file_ids_item_data in _attachment_file_ids:
                attachment_file_ids_item = UUID(attachment_file_ids_item_data)

                attachment_file_ids.append(attachment_file_ids_item)

        _mention_ids = d.pop("mention_ids", UNSET)
        mention_ids: list[UUID] | Unset = UNSET
        if _mention_ids is not UNSET:
            mention_ids = []
            for mention_ids_item_data in _mention_ids:
                mention_ids_item = UUID(mention_ids_item_data)

                mention_ids.append(mention_ids_item)

        silent = d.pop("silent", UNSET)

        clear_draft = d.pop("clear_draft", UNSET)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        _components = d.pop("components", UNSET)
        components: list[ComponentCreate] | Unset = UNSET
        if _components is not UNSET:
            components = []
            for components_item_data in _components:
                components_item = ComponentCreate.from_dict(components_item_data)

                components.append(components_item)

        message_create = cls(
            client_msg_id=client_msg_id,
            kind=kind,
            content_md=content_md,
            reply_to_message_id=reply_to_message_id,
            attachment_file_ids=attachment_file_ids,
            mention_ids=mention_ids,
            silent=silent,
            clear_draft=clear_draft,
            run_id=run_id,
            components=components,
        )

        return message_create
