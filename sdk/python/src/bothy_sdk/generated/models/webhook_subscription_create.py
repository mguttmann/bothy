from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="WebhookSubscriptionCreate")


@_attrs_define
class WebhookSubscriptionCreate:
    """
    Attributes:
        url (str): HTTPS URL; the host must be in `webhook.allowed_hosts`.
        event_types (list[str]):
        enable_hmac (bool | Unset): Also sign with Standard Webhooks `v1` (HMAC); the secret is returned once.
    """

    url: str
    event_types: list[str]
    enable_hmac: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        url = self.url

        event_types = self.event_types

        enable_hmac = self.enable_hmac

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "url": url,
                "event_types": event_types,
            }
        )
        if enable_hmac is not UNSET:
            field_dict["enable_hmac"] = enable_hmac

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        url = d.pop("url")

        event_types = cast(list[str], d.pop("event_types"))

        enable_hmac = d.pop("enable_hmac", UNSET)

        webhook_subscription_create = cls(
            url=url,
            event_types=event_types,
            enable_hmac=enable_hmac,
        )

        return webhook_subscription_create
