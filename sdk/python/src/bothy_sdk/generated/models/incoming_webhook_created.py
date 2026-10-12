from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.incoming_webhook import IncomingWebhook


T = TypeVar("T", bound="IncomingWebhookCreated")


@_attrs_define
class IncomingWebhookCreated:
    """
    Attributes:
        webhook (IncomingWebhook):
        url (str): Full URL including the token. Shown once.
    """

    webhook: IncomingWebhook
    url: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        webhook = self.webhook.to_dict()

        url = self.url

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "webhook": webhook,
                "url": url,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.incoming_webhook import IncomingWebhook

        d = dict(src_dict)
        webhook = IncomingWebhook.from_dict(d.pop("webhook"))

        url = d.pop("url")

        incoming_webhook_created = cls(
            webhook=webhook,
            url=url,
        )

        incoming_webhook_created.additional_properties = d
        return incoming_webhook_created

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
