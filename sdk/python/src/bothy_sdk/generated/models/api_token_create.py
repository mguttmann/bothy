from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.app_scope import AppScope
from ..types import UNSET, Unset

T = TypeVar("T", bound="ApiTokenCreate")


@_attrs_define
class ApiTokenCreate:
    """
    Attributes:
        name (str):
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        scopes (list[AppScope] | Unset): Only for app tokens, a subset of the granted scopes of the installation. Agent
            tokens (all agent scopes implicitly) and personal tokens (the grants of the person) carry none.
        dpop_jkt (str | Unset): RFC 7638 thumbprint (43 base64url characters) of the key the token is bound to; requests
            must then use DPoP. Required for the kinds in `auth.dpop_required_for`.
    """

    name: str
    expires_at: datetime.datetime
    scopes: list[AppScope] | Unset = UNSET
    dpop_jkt: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        expires_at = self.expires_at.isoformat()

        scopes: list[str] | Unset = UNSET
        if not isinstance(self.scopes, Unset):
            scopes = []
            for scopes_item_data in self.scopes:
                scopes_item = scopes_item_data.value
                scopes.append(scopes_item)

        dpop_jkt = self.dpop_jkt

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "expires_at": expires_at,
            }
        )
        if scopes is not UNSET:
            field_dict["scopes"] = scopes
        if dpop_jkt is not UNSET:
            field_dict["dpop_jkt"] = dpop_jkt

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _scopes = d.pop("scopes", UNSET)
        scopes: list[AppScope] | Unset = UNSET
        if _scopes is not UNSET:
            scopes = []
            for scopes_item_data in _scopes:
                scopes_item = AppScope(scopes_item_data)

                scopes.append(scopes_item)

        dpop_jkt = d.pop("dpop_jkt", UNSET)

        api_token_create = cls(
            name=name,
            expires_at=expires_at,
            scopes=scopes,
            dpop_jkt=dpop_jkt,
        )

        return api_token_create
