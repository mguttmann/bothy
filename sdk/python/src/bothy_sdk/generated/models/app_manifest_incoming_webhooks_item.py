from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="AppManifestIncomingWebhooksItem")


@_attrs_define
class AppManifestIncomingWebhooksItem:
    """
    Attributes:
        name (str):
        purpose (str):
    """

    name: str
    purpose: str

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        purpose = self.purpose

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "purpose": purpose,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        purpose = d.pop("purpose")

        app_manifest_incoming_webhooks_item = cls(
            name=name,
            purpose=purpose,
        )

        return app_manifest_incoming_webhooks_item
