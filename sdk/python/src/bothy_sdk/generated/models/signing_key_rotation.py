from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.signing_key_purpose import SigningKeyPurpose

T = TypeVar("T", bound="SigningKeyRotation")


@_attrs_define
class SigningKeyRotation:
    """
    Attributes:
        purpose (SigningKeyPurpose): Purpose of a server signing key (03 `server_signing_keys.purpose`). `terminal`
            signs terminal session tokens (24 section 5.3); `audit_checkpoint` signs audit checkpoints (19 SEC-AUD-7).
        new_kid (str):
        previous_kid (str):
        previous_not_after (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    purpose: SigningKeyPurpose
    new_kid: str
    previous_kid: str
    previous_not_after: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        purpose = self.purpose.value

        new_kid = self.new_kid

        previous_kid = self.previous_kid

        previous_not_after = self.previous_not_after.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "purpose": purpose,
                "new_kid": new_kid,
                "previous_kid": previous_kid,
                "previous_not_after": previous_not_after,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        purpose = SigningKeyPurpose(d.pop("purpose"))

        new_kid = d.pop("new_kid")

        previous_kid = d.pop("previous_kid")

        previous_not_after = datetime.datetime.fromisoformat(
            d.pop("previous_not_after")
        )

        signing_key_rotation = cls(
            purpose=purpose,
            new_kid=new_kid,
            previous_kid=previous_kid,
            previous_not_after=previous_not_after,
        )

        signing_key_rotation.additional_properties = d
        return signing_key_rotation

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
