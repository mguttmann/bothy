from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="IncomingWebhook")


@_attrs_define
class IncomingWebhook:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        last_used_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        revoked_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    app_id: UUID
    channel_id: UUID
    name: str
    created_at: datetime.datetime
    thread_id: UUID | Unset = UNSET
    last_used_at: datetime.datetime | Unset = UNSET
    revoked_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        app_id = str(self.app_id)

        channel_id = str(self.channel_id)

        name = self.name

        created_at = self.created_at.isoformat()

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        last_used_at: str | Unset = UNSET
        if not isinstance(self.last_used_at, Unset):
            last_used_at = self.last_used_at.isoformat()

        revoked_at: str | Unset = UNSET
        if not isinstance(self.revoked_at, Unset):
            revoked_at = self.revoked_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "app_id": app_id,
                "channel_id": channel_id,
                "name": name,
                "created_at": created_at,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if last_used_at is not UNSET:
            field_dict["last_used_at"] = last_used_at
        if revoked_at is not UNSET:
            field_dict["revoked_at"] = revoked_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        app_id = UUID(d.pop("app_id"))

        channel_id = UUID(d.pop("channel_id"))

        name = d.pop("name")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _last_used_at = d.pop("last_used_at", UNSET)
        last_used_at: datetime.datetime | Unset
        if isinstance(_last_used_at, Unset):
            last_used_at = UNSET
        else:
            last_used_at = datetime.datetime.fromisoformat(_last_used_at)

        _revoked_at = d.pop("revoked_at", UNSET)
        revoked_at: datetime.datetime | Unset
        if isinstance(_revoked_at, Unset):
            revoked_at = UNSET
        else:
            revoked_at = datetime.datetime.fromisoformat(_revoked_at)

        incoming_webhook = cls(
            id=id,
            app_id=app_id,
            channel_id=channel_id,
            name=name,
            created_at=created_at,
            thread_id=thread_id,
            last_used_at=last_used_at,
            revoked_at=revoked_at,
        )

        incoming_webhook.additional_properties = d
        return incoming_webhook

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
