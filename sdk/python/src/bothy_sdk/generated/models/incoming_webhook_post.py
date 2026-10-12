from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="IncomingWebhookPost")


@_attrs_define
class IncomingWebhookPost:
    """
    Attributes:
        content_md (str):
        client_msg_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        title (str | Unset): Title of a new thread when the webhook has no fixed thread.
    """

    content_md: str
    client_msg_id: UUID | Unset = UNSET
    title: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        content_md = self.content_md

        client_msg_id: str | Unset = UNSET
        if not isinstance(self.client_msg_id, Unset):
            client_msg_id = str(self.client_msg_id)

        title = self.title

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "content_md": content_md,
            }
        )
        if client_msg_id is not UNSET:
            field_dict["client_msg_id"] = client_msg_id
        if title is not UNSET:
            field_dict["title"] = title

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        content_md = d.pop("content_md")

        _client_msg_id = d.pop("client_msg_id", UNSET)
        client_msg_id: UUID | Unset
        if isinstance(_client_msg_id, Unset):
            client_msg_id = UNSET
        else:
            client_msg_id = UUID(_client_msg_id)

        title = d.pop("title", UNSET)

        incoming_webhook_post = cls(
            content_md=content_md,
            client_msg_id=client_msg_id,
            title=title,
        )

        return incoming_webhook_post
