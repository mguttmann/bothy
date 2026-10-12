from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.token_response_token_type import TokenResponseTokenType

if TYPE_CHECKING:
    from ..models.device import Device
    from ..models.session import Session


T = TypeVar("T", bound="TokenResponse")


@_attrs_define
class TokenResponse:
    """Token pair after login or refresh. `token_type` is always `DPoP` (RFC 9449).

    Attributes:
        access_token (str):
        token_type (TokenResponseTokenType):
        expires_in (int): Seconds until the access token expires.
        refresh_token (str):
        refresh_expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        session (Session):
        device (Device):
    """

    access_token: str
    token_type: TokenResponseTokenType
    expires_in: int
    refresh_token: str
    refresh_expires_at: datetime.datetime
    session: Session
    device: Device
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        access_token = self.access_token

        token_type = self.token_type.value

        expires_in = self.expires_in

        refresh_token = self.refresh_token

        refresh_expires_at = self.refresh_expires_at.isoformat()

        session = self.session.to_dict()

        device = self.device.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "access_token": access_token,
                "token_type": token_type,
                "expires_in": expires_in,
                "refresh_token": refresh_token,
                "refresh_expires_at": refresh_expires_at,
                "session": session,
                "device": device,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.device import Device
        from ..models.session import Session

        d = dict(src_dict)
        access_token = d.pop("access_token")

        token_type = TokenResponseTokenType(d.pop("token_type"))

        expires_in = d.pop("expires_in")

        refresh_token = d.pop("refresh_token")

        refresh_expires_at = datetime.datetime.fromisoformat(
            d.pop("refresh_expires_at")
        )

        session = Session.from_dict(d.pop("session"))

        device = Device.from_dict(d.pop("device"))

        token_response = cls(
            access_token=access_token,
            token_type=token_type,
            expires_in=expires_in,
            refresh_token=refresh_token,
            refresh_expires_at=refresh_expires_at,
            session=session,
            device=device,
        )

        token_response.additional_properties = d
        return token_response

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
