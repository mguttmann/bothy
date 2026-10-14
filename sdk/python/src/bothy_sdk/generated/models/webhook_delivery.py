from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.webhook_delivery_status import WebhookDeliveryStatus
from ..types import UNSET, Unset

T = TypeVar("T", bound="WebhookDelivery")


@_attrs_define
class WebhookDelivery:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        subscription_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        event_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        event_seq (int):
        event_type (str):
        attempt (int):
        status (WebhookDeliveryStatus):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        response_code (int | Unset):
        last_error (str | Unset):
        next_attempt_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        delivered_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    subscription_id: UUID
    event_id: UUID
    event_seq: int
    event_type: str
    attempt: int
    status: WebhookDeliveryStatus
    created_at: datetime.datetime
    response_code: int | Unset = UNSET
    last_error: str | Unset = UNSET
    next_attempt_at: datetime.datetime | Unset = UNSET
    delivered_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        subscription_id = str(self.subscription_id)

        event_id = str(self.event_id)

        event_seq = self.event_seq

        event_type = self.event_type

        attempt = self.attempt

        status = self.status.value

        created_at = self.created_at.isoformat()

        response_code = self.response_code

        last_error = self.last_error

        next_attempt_at: str | Unset = UNSET
        if not isinstance(self.next_attempt_at, Unset):
            next_attempt_at = self.next_attempt_at.isoformat()

        delivered_at: str | Unset = UNSET
        if not isinstance(self.delivered_at, Unset):
            delivered_at = self.delivered_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "subscription_id": subscription_id,
                "event_id": event_id,
                "event_seq": event_seq,
                "event_type": event_type,
                "attempt": attempt,
                "status": status,
                "created_at": created_at,
            }
        )
        if response_code is not UNSET:
            field_dict["response_code"] = response_code
        if last_error is not UNSET:
            field_dict["last_error"] = last_error
        if next_attempt_at is not UNSET:
            field_dict["next_attempt_at"] = next_attempt_at
        if delivered_at is not UNSET:
            field_dict["delivered_at"] = delivered_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        subscription_id = UUID(d.pop("subscription_id"))

        event_id = UUID(d.pop("event_id"))

        event_seq = d.pop("event_seq")

        event_type = d.pop("event_type")

        attempt = d.pop("attempt")

        status = WebhookDeliveryStatus(d.pop("status"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        response_code = d.pop("response_code", UNSET)

        last_error = d.pop("last_error", UNSET)

        _next_attempt_at = d.pop("next_attempt_at", UNSET)
        next_attempt_at: datetime.datetime | Unset
        if isinstance(_next_attempt_at, Unset):
            next_attempt_at = UNSET
        else:
            next_attempt_at = datetime.datetime.fromisoformat(_next_attempt_at)

        _delivered_at = d.pop("delivered_at", UNSET)
        delivered_at: datetime.datetime | Unset
        if isinstance(_delivered_at, Unset):
            delivered_at = UNSET
        else:
            delivered_at = datetime.datetime.fromisoformat(_delivered_at)

        webhook_delivery = cls(
            id=id,
            subscription_id=subscription_id,
            event_id=event_id,
            event_seq=event_seq,
            event_type=event_type,
            attempt=attempt,
            status=status,
            created_at=created_at,
            response_code=response_code,
            last_error=last_error,
            next_attempt_at=next_attempt_at,
            delivered_at=delivered_at,
        )

        webhook_delivery.additional_properties = d
        return webhook_delivery

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
