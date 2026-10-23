from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.push_environment import PushEnvironment
from ..models.push_provider import PushProvider
from ..types import UNSET, Unset

T = TypeVar("T", bound="PushRegistrationCreated")


@_attrs_define
class PushRegistrationCreated:
    """A new push registration with its preview token, which is shown exactly once and never again (13, 6.4).

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        provider (PushProvider):
        environment (PushEnvironment):
        app_id (str):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        preview_token (str): Preview token of the registration (`bothy_pvw_...`), shown exactly once. The app keeps it
            in the store it shares with its notification extension.
        disabled_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        disabled_reason (str | Unset):
    """

    id: UUID
    device_id: UUID
    provider: PushProvider
    environment: PushEnvironment
    app_id: str
    created_at: datetime.datetime
    preview_token: str
    disabled_at: datetime.datetime | Unset = UNSET
    disabled_reason: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        device_id = str(self.device_id)

        provider = self.provider.value

        environment = self.environment.value

        app_id = self.app_id

        created_at = self.created_at.isoformat()

        preview_token = self.preview_token

        disabled_at: str | Unset = UNSET
        if not isinstance(self.disabled_at, Unset):
            disabled_at = self.disabled_at.isoformat()

        disabled_reason = self.disabled_reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "device_id": device_id,
                "provider": provider,
                "environment": environment,
                "app_id": app_id,
                "created_at": created_at,
                "preview_token": preview_token,
            }
        )
        if disabled_at is not UNSET:
            field_dict["disabled_at"] = disabled_at
        if disabled_reason is not UNSET:
            field_dict["disabled_reason"] = disabled_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        device_id = UUID(d.pop("device_id"))

        provider = PushProvider(d.pop("provider"))

        environment = PushEnvironment(d.pop("environment"))

        app_id = d.pop("app_id")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        preview_token = d.pop("preview_token")

        _disabled_at = d.pop("disabled_at", UNSET)
        disabled_at: datetime.datetime | Unset
        if isinstance(_disabled_at, Unset):
            disabled_at = UNSET
        else:
            disabled_at = datetime.datetime.fromisoformat(_disabled_at)

        disabled_reason = d.pop("disabled_reason", UNSET)

        push_registration_created = cls(
            id=id,
            device_id=device_id,
            provider=provider,
            environment=environment,
            app_id=app_id,
            created_at=created_at,
            preview_token=preview_token,
            disabled_at=disabled_at,
            disabled_reason=disabled_reason,
        )

        push_registration_created.additional_properties = d
        return push_registration_created

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
