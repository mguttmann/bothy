from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.webhook_subscription_signature_schemes_item import (
    WebhookSubscriptionSignatureSchemesItem,
)
from ..models.webhook_subscription_state import WebhookSubscriptionState
from ..types import UNSET, Unset

T = TypeVar("T", bound="WebhookSubscription")


@_attrs_define
class WebhookSubscription:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        url (str):
        event_types (list[str]):
        state (WebhookSubscriptionState):
        consecutive_failures (int):
        signature_schemes (list[WebhookSubscriptionSignatureSchemesItem]):
        signing_key_kid (str):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        app_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        disabled_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    url: str
    event_types: list[str]
    state: WebhookSubscriptionState
    consecutive_failures: int
    signature_schemes: list[WebhookSubscriptionSignatureSchemesItem]
    signing_key_kid: str
    created_at: datetime.datetime
    app_id: UUID | Unset = UNSET
    agent_id: UUID | Unset = UNSET
    disabled_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        url = self.url

        event_types = self.event_types

        state = self.state.value

        consecutive_failures = self.consecutive_failures

        signature_schemes = []
        for signature_schemes_item_data in self.signature_schemes:
            signature_schemes_item = signature_schemes_item_data.value
            signature_schemes.append(signature_schemes_item)

        signing_key_kid = self.signing_key_kid

        created_at = self.created_at.isoformat()

        app_id: str | Unset = UNSET
        if not isinstance(self.app_id, Unset):
            app_id = str(self.app_id)

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        disabled_at: str | Unset = UNSET
        if not isinstance(self.disabled_at, Unset):
            disabled_at = self.disabled_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "url": url,
                "event_types": event_types,
                "state": state,
                "consecutive_failures": consecutive_failures,
                "signature_schemes": signature_schemes,
                "signing_key_kid": signing_key_kid,
                "created_at": created_at,
            }
        )
        if app_id is not UNSET:
            field_dict["app_id"] = app_id
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if disabled_at is not UNSET:
            field_dict["disabled_at"] = disabled_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        url = d.pop("url")

        event_types = cast(list[str], d.pop("event_types"))

        state = WebhookSubscriptionState(d.pop("state"))

        consecutive_failures = d.pop("consecutive_failures")

        signature_schemes = []
        _signature_schemes = d.pop("signature_schemes")
        for signature_schemes_item_data in _signature_schemes:
            signature_schemes_item = WebhookSubscriptionSignatureSchemesItem(
                signature_schemes_item_data
            )

            signature_schemes.append(signature_schemes_item)

        signing_key_kid = d.pop("signing_key_kid")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _app_id = d.pop("app_id", UNSET)
        app_id: UUID | Unset
        if isinstance(_app_id, Unset):
            app_id = UNSET
        else:
            app_id = UUID(_app_id)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _disabled_at = d.pop("disabled_at", UNSET)
        disabled_at: datetime.datetime | Unset
        if isinstance(_disabled_at, Unset):
            disabled_at = UNSET
        else:
            disabled_at = datetime.datetime.fromisoformat(_disabled_at)

        webhook_subscription = cls(
            id=id,
            url=url,
            event_types=event_types,
            state=state,
            consecutive_failures=consecutive_failures,
            signature_schemes=signature_schemes,
            signing_key_kid=signing_key_kid,
            created_at=created_at,
            app_id=app_id,
            agent_id=agent_id,
            disabled_at=disabled_at,
        )

        webhook_subscription.additional_properties = d
        return webhook_subscription

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
