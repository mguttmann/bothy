from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.app_installation_state import AppInstallationState
from ..models.app_scope import AppScope
from ..types import UNSET, Unset

T = TypeVar("T", bound="AppInstallation")


@_attrs_define
class AppInstallation:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        granted_scopes (list[AppScope]):
        event_subscriptions (list[str]):
        state (AppInstallationState):
        installed_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        installed_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    id: UUID
    granted_scopes: list[AppScope]
    event_subscriptions: list[str]
    state: AppInstallationState
    installed_at: datetime.datetime
    installed_by: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        granted_scopes = []
        for granted_scopes_item_data in self.granted_scopes:
            granted_scopes_item = granted_scopes_item_data.value
            granted_scopes.append(granted_scopes_item)

        event_subscriptions = self.event_subscriptions

        state = self.state.value

        installed_at = self.installed_at.isoformat()

        installed_by: str | Unset = UNSET
        if not isinstance(self.installed_by, Unset):
            installed_by = str(self.installed_by)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "granted_scopes": granted_scopes,
                "event_subscriptions": event_subscriptions,
                "state": state,
                "installed_at": installed_at,
            }
        )
        if installed_by is not UNSET:
            field_dict["installed_by"] = installed_by

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        granted_scopes = []
        _granted_scopes = d.pop("granted_scopes")
        for granted_scopes_item_data in _granted_scopes:
            granted_scopes_item = AppScope(granted_scopes_item_data)

            granted_scopes.append(granted_scopes_item)

        event_subscriptions = cast(list[str], d.pop("event_subscriptions"))

        state = AppInstallationState(d.pop("state"))

        installed_at = datetime.datetime.fromisoformat(d.pop("installed_at"))

        _installed_by = d.pop("installed_by", UNSET)
        installed_by: UUID | Unset
        if isinstance(_installed_by, Unset):
            installed_by = UNSET
        else:
            installed_by = UUID(_installed_by)

        app_installation = cls(
            id=id,
            granted_scopes=granted_scopes,
            event_subscriptions=event_subscriptions,
            state=state,
            installed_at=installed_at,
            installed_by=installed_by,
        )

        app_installation.additional_properties = d
        return app_installation

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
