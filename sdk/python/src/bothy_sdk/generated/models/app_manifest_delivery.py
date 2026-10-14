from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.app_manifest_delivery_mode import AppManifestDeliveryMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="AppManifestDelivery")


@_attrs_define
class AppManifestDelivery:
    """Required with `events`, `commands` or `components`.

    Attributes:
        mode (AppManifestDeliveryMode):
        events_url (str | Unset): Required for `webhook`, not allowed for `gateway`; `https`, host in
            `webhook.allowed_hosts`.
        interactions_url (str | Unset): Required for commands or components without the gateway; `https`, host in
            `webhook.allowed_hosts`.
    """

    mode: AppManifestDeliveryMode
    events_url: str | Unset = UNSET
    interactions_url: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        mode = self.mode.value

        events_url = self.events_url

        interactions_url = self.interactions_url

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "mode": mode,
            }
        )
        if events_url is not UNSET:
            field_dict["events_url"] = events_url
        if interactions_url is not UNSET:
            field_dict["interactions_url"] = interactions_url

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        mode = AppManifestDeliveryMode(d.pop("mode"))

        events_url = d.pop("events_url", UNSET)

        interactions_url = d.pop("interactions_url", UNSET)

        app_manifest_delivery = cls(
            mode=mode,
            events_url=events_url,
            interactions_url=interactions_url,
        )

        return app_manifest_delivery
