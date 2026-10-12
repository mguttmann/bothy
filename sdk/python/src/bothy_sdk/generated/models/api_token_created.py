from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.api_token import ApiToken


T = TypeVar("T", bound="ApiTokenCreated")


@_attrs_define
class ApiTokenCreated:
    """
    Attributes:
        token (ApiToken):
        secret (str): Full token value `bothy_<kind>_<random>_<checksum>` (prefix for secret scanners). Shown exactly
            once.
    """

    token: ApiToken
    secret: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        token = self.token.to_dict()

        secret = self.secret

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "token": token,
                "secret": secret,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.api_token import ApiToken

        d = dict(src_dict)
        token = ApiToken.from_dict(d.pop("token"))

        secret = d.pop("secret")

        api_token_created = cls(
            token=token,
            secret=secret,
        )

        api_token_created.additional_properties = d
        return api_token_created

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
