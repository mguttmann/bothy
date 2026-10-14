from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="Me")


@_attrs_define
class Me:
    """The calling principal.

    Attributes:
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        is_owner (bool):
        permission_version (int):
        email (str | Unset):
        locale (str | Unset):
        time_zone (str | Unset):
        session_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        step_up_valid_until (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
    """

    principal: PrincipalRef
    is_owner: bool
    permission_version: int
    email: str | Unset = UNSET
    locale: str | Unset = UNSET
    time_zone: str | Unset = UNSET
    session_id: UUID | Unset = UNSET
    device_id: UUID | Unset = UNSET
    step_up_valid_until: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        principal = self.principal.to_dict()

        is_owner = self.is_owner

        permission_version = self.permission_version

        email = self.email

        locale = self.locale

        time_zone = self.time_zone

        session_id: str | Unset = UNSET
        if not isinstance(self.session_id, Unset):
            session_id = str(self.session_id)

        device_id: str | Unset = UNSET
        if not isinstance(self.device_id, Unset):
            device_id = str(self.device_id)

        step_up_valid_until: str | Unset = UNSET
        if not isinstance(self.step_up_valid_until, Unset):
            step_up_valid_until = self.step_up_valid_until.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "principal": principal,
                "is_owner": is_owner,
                "permission_version": permission_version,
            }
        )
        if email is not UNSET:
            field_dict["email"] = email
        if locale is not UNSET:
            field_dict["locale"] = locale
        if time_zone is not UNSET:
            field_dict["time_zone"] = time_zone
        if session_id is not UNSET:
            field_dict["session_id"] = session_id
        if device_id is not UNSET:
            field_dict["device_id"] = device_id
        if step_up_valid_until is not UNSET:
            field_dict["step_up_valid_until"] = step_up_valid_until

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        principal = PrincipalRef.from_dict(d.pop("principal"))

        is_owner = d.pop("is_owner")

        permission_version = d.pop("permission_version")

        email = d.pop("email", UNSET)

        locale = d.pop("locale", UNSET)

        time_zone = d.pop("time_zone", UNSET)

        _session_id = d.pop("session_id", UNSET)
        session_id: UUID | Unset
        if isinstance(_session_id, Unset):
            session_id = UNSET
        else:
            session_id = UUID(_session_id)

        _device_id = d.pop("device_id", UNSET)
        device_id: UUID | Unset
        if isinstance(_device_id, Unset):
            device_id = UNSET
        else:
            device_id = UUID(_device_id)

        _step_up_valid_until = d.pop("step_up_valid_until", UNSET)
        step_up_valid_until: datetime.datetime | Unset
        if isinstance(_step_up_valid_until, Unset):
            step_up_valid_until = UNSET
        else:
            step_up_valid_until = datetime.datetime.fromisoformat(_step_up_valid_until)

        me = cls(
            principal=principal,
            is_owner=is_owner,
            permission_version=permission_version,
            email=email,
            locale=locale,
            time_zone=time_zone,
            session_id=session_id,
            device_id=device_id,
            step_up_valid_until=step_up_valid_until,
        )

        me.additional_properties = d
        return me

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
