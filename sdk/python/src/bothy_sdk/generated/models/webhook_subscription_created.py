from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.webhook_subscription import WebhookSubscription


T = TypeVar("T", bound="WebhookSubscriptionCreated")


@_attrs_define
class WebhookSubscriptionCreated:
    """
    Attributes:
        subscription (WebhookSubscription):
        hmac_secret (str | Unset): Standard Webhooks secret with prefix `whsec_`. Shown once.
    """

    subscription: WebhookSubscription
    hmac_secret: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        subscription = self.subscription.to_dict()

        hmac_secret = self.hmac_secret

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "subscription": subscription,
            }
        )
        if hmac_secret is not UNSET:
            field_dict["hmac_secret"] = hmac_secret

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.webhook_subscription import WebhookSubscription

        d = dict(src_dict)
        subscription = WebhookSubscription.from_dict(d.pop("subscription"))

        hmac_secret = d.pop("hmac_secret", UNSET)

        webhook_subscription_created = cls(
            subscription=subscription,
            hmac_secret=hmac_secret,
        )

        webhook_subscription_created.additional_properties = d
        return webhook_subscription_created

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
