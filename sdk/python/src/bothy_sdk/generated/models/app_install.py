from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.app_scope import AppScope
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.app_manifest import AppManifest


T = TypeVar("T", bound="AppInstall")


@_attrs_define
class AppInstall:
    """
    Attributes:
        manifest (AppManifest): Schema `app_manifest.v1`.
        granted_scopes (list[AppScope]):
        reason (str):
        event_subscriptions (list[str] | Unset):
    """

    manifest: AppManifest
    granted_scopes: list[AppScope]
    reason: str
    event_subscriptions: list[str] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        manifest = self.manifest.to_dict()

        granted_scopes = []
        for granted_scopes_item_data in self.granted_scopes:
            granted_scopes_item = granted_scopes_item_data.value
            granted_scopes.append(granted_scopes_item)

        reason = self.reason

        event_subscriptions: list[str] | Unset = UNSET
        if not isinstance(self.event_subscriptions, Unset):
            event_subscriptions = self.event_subscriptions

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "manifest": manifest,
                "granted_scopes": granted_scopes,
                "reason": reason,
            }
        )
        if event_subscriptions is not UNSET:
            field_dict["event_subscriptions"] = event_subscriptions

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.app_manifest import AppManifest

        d = dict(src_dict)
        manifest = AppManifest.from_dict(d.pop("manifest"))

        granted_scopes = []
        _granted_scopes = d.pop("granted_scopes")
        for granted_scopes_item_data in _granted_scopes:
            granted_scopes_item = AppScope(granted_scopes_item_data)

            granted_scopes.append(granted_scopes_item)

        reason = d.pop("reason")

        event_subscriptions = cast(list[str], d.pop("event_subscriptions", UNSET))

        app_install = cls(
            manifest=manifest,
            granted_scopes=granted_scopes,
            reason=reason,
            event_subscriptions=event_subscriptions,
        )

        return app_install
