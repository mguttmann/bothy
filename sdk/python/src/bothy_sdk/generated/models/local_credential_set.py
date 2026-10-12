from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="LocalCredentialSet")


@_attrs_define
class LocalCredentialSet:
    """Sets or replaces the local password, starts a TOTP enrollment (`totp_enroll`, the response carries the new secret
    once) or confirms it (`totp_code`). At least one member; the first call sets `password`. The password path counts as
    established once TOTP is confirmed (08, 2.5).

        Attributes:
            password (str | Unset):
            totp_enroll (bool | Unset):
            totp_code (str | Unset): Current code of the enrolled TOTP secret; confirms the enrollment.
    """

    password: str | Unset = UNSET
    totp_enroll: bool | Unset = UNSET
    totp_code: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        password = self.password

        totp_enroll = self.totp_enroll

        totp_code = self.totp_code

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if password is not UNSET:
            field_dict["password"] = password
        if totp_enroll is not UNSET:
            field_dict["totp_enroll"] = totp_enroll
        if totp_code is not UNSET:
            field_dict["totp_code"] = totp_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        password = d.pop("password", UNSET)

        totp_enroll = d.pop("totp_enroll", UNSET)

        totp_code = d.pop("totp_code", UNSET)

        local_credential_set = cls(
            password=password,
            totp_enroll=totp_enroll,
            totp_code=totp_code,
        )

        return local_credential_set
