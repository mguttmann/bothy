from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="LocalCredentialResult")


@_attrs_define
class LocalCredentialResult:
    """
    Attributes:
        password_set (bool):
        totp_enabled (bool): True once a TOTP enrollment is confirmed.
        totp_secret_uri (str | Unset): otpauth URI, returned once when `totp_enroll` was set.
    """

    password_set: bool
    totp_enabled: bool
    totp_secret_uri: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        password_set = self.password_set

        totp_enabled = self.totp_enabled

        totp_secret_uri = self.totp_secret_uri

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "password_set": password_set,
                "totp_enabled": totp_enabled,
            }
        )
        if totp_secret_uri is not UNSET:
            field_dict["totp_secret_uri"] = totp_secret_uri

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        password_set = d.pop("password_set")

        totp_enabled = d.pop("totp_enabled")

        totp_secret_uri = d.pop("totp_secret_uri", UNSET)

        local_credential_result = cls(
            password_set=password_set,
            totp_enabled=totp_enabled,
            totp_secret_uri=totp_secret_uri,
        )

        local_credential_result.additional_properties = d
        return local_credential_result

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
