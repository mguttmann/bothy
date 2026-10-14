from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="IdentityLinkComplete")


@_attrs_define
class IdentityLinkComplete:
    """
    Attributes:
        login_code (str):
        client_verifier (str):
    """

    login_code: str
    client_verifier: str

    def to_dict(self) -> dict[str, Any]:
        login_code = self.login_code

        client_verifier = self.client_verifier

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "login_code": login_code,
                "client_verifier": client_verifier,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        login_code = d.pop("login_code")

        client_verifier = d.pop("client_verifier")

        identity_link_complete = cls(
            login_code=login_code,
            client_verifier=client_verifier,
        )

        return identity_link_complete
