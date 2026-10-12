from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.identity import Identity
    from ..models.passkey import Passkey


T = TypeVar("T", bound="CredentialOverview")


@_attrs_define
class CredentialOverview:
    """
    Attributes:
        identities (list[Identity]):
        passkeys (list[Passkey]):
        local_password_set (bool):
        totp_enrolled (bool):
    """

    identities: list[Identity]
    passkeys: list[Passkey]
    local_password_set: bool
    totp_enrolled: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        identities = []
        for identities_item_data in self.identities:
            identities_item = identities_item_data.to_dict()
            identities.append(identities_item)

        passkeys = []
        for passkeys_item_data in self.passkeys:
            passkeys_item = passkeys_item_data.to_dict()
            passkeys.append(passkeys_item)

        local_password_set = self.local_password_set

        totp_enrolled = self.totp_enrolled

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "identities": identities,
                "passkeys": passkeys,
                "local_password_set": local_password_set,
                "totp_enrolled": totp_enrolled,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.identity import Identity
        from ..models.passkey import Passkey

        d = dict(src_dict)
        identities = []
        _identities = d.pop("identities")
        for identities_item_data in _identities:
            identities_item = Identity.from_dict(identities_item_data)

            identities.append(identities_item)

        passkeys = []
        _passkeys = d.pop("passkeys")
        for passkeys_item_data in _passkeys:
            passkeys_item = Passkey.from_dict(passkeys_item_data)

            passkeys.append(passkeys_item)

        local_password_set = d.pop("local_password_set")

        totp_enrolled = d.pop("totp_enrolled")

        credential_overview = cls(
            identities=identities,
            passkeys=passkeys,
            local_password_set=local_password_set,
            totp_enrolled=totp_enrolled,
        )

        credential_overview.additional_properties = d
        return credential_overview

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
