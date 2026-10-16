from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="StepUpPasswordRequest")


@_attrs_define
class StepUpPasswordRequest:
    """
    Attributes:
        challenge_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        password (str):
        totp_code (str):
    """

    challenge_id: UUID
    password: str
    totp_code: str

    def to_dict(self) -> dict[str, Any]:
        challenge_id = str(self.challenge_id)

        password = self.password

        totp_code = self.totp_code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "challenge_id": challenge_id,
                "password": password,
                "totp_code": totp_code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        challenge_id = UUID(d.pop("challenge_id"))

        password = d.pop("password")

        totp_code = d.pop("totp_code")

        step_up_password_request = cls(
            challenge_id=challenge_id,
            password=password,
            totp_code=totp_code,
        )

        return step_up_password_request
