from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="OidcBeginRequest")


@_attrs_define
class OidcBeginRequest:
    """
    Attributes:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        app_redirect_uri (str): Redirect target of the app (loopback or claimed scheme, RFC 8252). Must match the allow-
            list of the provider.
        client_challenge (str): Base64url SHA-256 of a client verifier, binds the later `login_code` to this app
            instance.
        enrollment_code (str | Unset): Optional enrollment code (purpose `first_login`, `recovery` or `identity_link`).
            Checked at begin, consumed at complete; an unknown subject is then linked to the person of the code (08, 3.1).
    """

    provider_id: UUID
    app_redirect_uri: str
    client_challenge: str
    enrollment_code: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        provider_id = str(self.provider_id)

        app_redirect_uri = self.app_redirect_uri

        client_challenge = self.client_challenge

        enrollment_code = self.enrollment_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "provider_id": provider_id,
                "app_redirect_uri": app_redirect_uri,
                "client_challenge": client_challenge,
            }
        )
        if enrollment_code is not UNSET:
            field_dict["enrollment_code"] = enrollment_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        provider_id = UUID(d.pop("provider_id"))

        app_redirect_uri = d.pop("app_redirect_uri")

        client_challenge = d.pop("client_challenge")

        enrollment_code = d.pop("enrollment_code", UNSET)

        oidc_begin_request = cls(
            provider_id=provider_id,
            app_redirect_uri=app_redirect_uri,
            client_challenge=client_challenge,
            enrollment_code=enrollment_code,
        )

        return oidc_begin_request
