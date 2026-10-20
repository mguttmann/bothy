from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="IncomingWebhookCreate")


@_attrs_define
class IncomingWebhookCreate:
    """Body of `createIncomingWebhook`. `app_id` names an app principal whose installation is
    `active` (else `400 validation_failed` with `errors[].code` `app_not_installed`) and that may
    write in the channel (else `app_cannot_write`); `thread_id`, when present, is a thread of this
    channel (else `not_in_channel`, an unknown ID too). `name` is not empty and has no control or
    bidi characters.

        Attributes:
            app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            name (str):
            thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    app_id: UUID
    name: str
    thread_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        app_id = str(self.app_id)

        name = self.name

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "app_id": app_id,
                "name": name,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        app_id = UUID(d.pop("app_id"))

        name = d.pop("name")

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        incoming_webhook_create = cls(
            app_id=app_id,
            name=name,
            thread_id=thread_id,
        )

        return incoming_webhook_create
