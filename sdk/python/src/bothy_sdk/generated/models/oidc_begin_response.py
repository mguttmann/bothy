from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="OidcBeginResponse")


@_attrs_define
class OidcBeginResponse:
    """
    Attributes:
        login_attempt_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        authorization_url (str):
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    login_attempt_id: UUID
    authorization_url: str
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        login_attempt_id = str(self.login_attempt_id)

        authorization_url = self.authorization_url

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "login_attempt_id": login_attempt_id,
                "authorization_url": authorization_url,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        login_attempt_id = UUID(d.pop("login_attempt_id"))

        authorization_url = d.pop("authorization_url")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        oidc_begin_response = cls(
            login_attempt_id=login_attempt_id,
            authorization_url=authorization_url,
            expires_at=expires_at,
        )

        oidc_begin_response.additional_properties = d
        return oidc_begin_response

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
