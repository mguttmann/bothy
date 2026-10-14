from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.webhook_subscription_update_state import WebhookSubscriptionUpdateState
from ..types import UNSET, Unset

T = TypeVar("T", bound="WebhookSubscriptionUpdate")


@_attrs_define
class WebhookSubscriptionUpdate:
    """
    Attributes:
        url (str | Unset):
        event_types (list[str] | Unset):
        state (WebhookSubscriptionUpdateState | Unset):
        rotate_hmac_secret (bool | Unset):
    """

    url: str | Unset = UNSET
    event_types: list[str] | Unset = UNSET
    state: WebhookSubscriptionUpdateState | Unset = UNSET
    rotate_hmac_secret: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        url = self.url

        event_types: list[str] | Unset = UNSET
        if not isinstance(self.event_types, Unset):
            event_types = self.event_types

        state: str | Unset = UNSET
        if not isinstance(self.state, Unset):
            state = self.state.value

        rotate_hmac_secret = self.rotate_hmac_secret

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if url is not UNSET:
            field_dict["url"] = url
        if event_types is not UNSET:
            field_dict["event_types"] = event_types
        if state is not UNSET:
            field_dict["state"] = state
        if rotate_hmac_secret is not UNSET:
            field_dict["rotate_hmac_secret"] = rotate_hmac_secret

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        url = d.pop("url", UNSET)

        event_types = cast(list[str], d.pop("event_types", UNSET))

        _state = d.pop("state", UNSET)
        state: WebhookSubscriptionUpdateState | Unset
        if isinstance(_state, Unset):
            state = UNSET
        else:
            state = WebhookSubscriptionUpdateState(_state)

        rotate_hmac_secret = d.pop("rotate_hmac_secret", UNSET)

        webhook_subscription_update = cls(
            url=url,
            event_types=event_types,
            state=state,
            rotate_hmac_secret=rotate_hmac_secret,
        )

        return webhook_subscription_update
