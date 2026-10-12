from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="StepUpChallenge")


@_attrs_define
class StepUpChallenge:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        challenge (str): Base64url random value to sign. Single use, valid `auth.step_up_challenge_ttl`. Clients show
            their own localized prompt text.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        passkey_options (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    challenge_id: UUID
    challenge: str
    expires_at: datetime.datetime
    passkey_options: JsonObject | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        challenge = self.challenge

        expires_at = self.expires_at.isoformat()

        passkey_options: dict[str, Any] | Unset = UNSET
        if not isinstance(self.passkey_options, Unset):
            passkey_options = self.passkey_options.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "challenge_id": challenge_id,
                "challenge": challenge,
                "expires_at": expires_at,
            }
        )
        if passkey_options is not UNSET:
            field_dict["passkey_options"] = passkey_options

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        challenge = d.pop("challenge")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _passkey_options = d.pop("passkey_options", UNSET)
        passkey_options: JsonObject | Unset
        if isinstance(_passkey_options, Unset):
            passkey_options = UNSET
        else:
            passkey_options = JsonObject.from_dict(_passkey_options)

        step_up_challenge = cls(
            challenge_id=challenge_id,
            challenge=challenge,
            expires_at=expires_at,
            passkey_options=passkey_options,
        )

        step_up_challenge.additional_properties = d
        return step_up_challenge

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
