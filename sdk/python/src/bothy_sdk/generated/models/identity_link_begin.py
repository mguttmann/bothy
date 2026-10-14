from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="IdentityLinkBegin")


@_attrs_define
class IdentityLinkBegin:
    """
    Attributes:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        app_redirect_uri (str):
        client_challenge (str):
    """

    provider_id: UUID
    app_redirect_uri: str
    client_challenge: str

    def to_dict(self) -> dict[str, Any]:
        provider_id = str(self.provider_id)

        app_redirect_uri = self.app_redirect_uri

        client_challenge = self.client_challenge

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "provider_id": provider_id,
                "app_redirect_uri": app_redirect_uri,
                "client_challenge": client_challenge,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        provider_id = UUID(d.pop("provider_id"))

        app_redirect_uri = d.pop("app_redirect_uri")

        client_challenge = d.pop("client_challenge")

        identity_link_begin = cls(
            provider_id=provider_id,
            app_redirect_uri=app_redirect_uri,
            client_challenge=client_challenge,
        )

        return identity_link_begin
