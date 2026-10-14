from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.push_environment import PushEnvironment
from ..models.push_provider import PushProvider
from ..types import UNSET, Unset

T = TypeVar("T", bound="PushRegistrationCreate")


@_attrs_define
class PushRegistrationCreate:
    """
    Attributes:
        provider (PushProvider):
        token (str): Push token or endpoint of the provider. Never returned.
        app_id (str):
        environment (PushEnvironment | Unset):
    """

    provider: PushProvider
    token: str
    app_id: str
    environment: PushEnvironment | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        provider = self.provider.value

        token = self.token

        app_id = self.app_id

        environment: str | Unset = UNSET
        if not isinstance(self.environment, Unset):
            environment = self.environment.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "provider": provider,
                "token": token,
                "app_id": app_id,
            }
        )
        if environment is not UNSET:
            field_dict["environment"] = environment

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        provider = PushProvider(d.pop("provider"))

        token = d.pop("token")

        app_id = d.pop("app_id")

        _environment = d.pop("environment", UNSET)
        environment: PushEnvironment | Unset
        if isinstance(_environment, Unset):
            environment = UNSET
        else:
            environment = PushEnvironment(_environment)

        push_registration_create = cls(
            provider=provider,
            token=token,
            app_id=app_id,
            environment=environment,
        )

        return push_registration_create
