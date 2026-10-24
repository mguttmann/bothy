from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.device_platform import DevicePlatform
from ..models.device_revoke_reason import DeviceRevokeReason
from ..models.device_state import DeviceState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.push_registration import PushRegistration


T = TypeVar("T", bound="Device")


@_attrs_define
class Device:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str):
        platform (DevicePlatform):
        state (DeviceState):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        app_version (str | Unset):
        key_thumbprint (str | Unset): RFC 7638 thumbprint of the active `dpop` device key (`cnf.jkt`).
        step_up_key_thumbprint (str | Unset): RFC 7638 thumbprint of the active `step_up` device key, if the device has
            one.
        verification_code (str | Unset): Only in responses to the pending device itself (`TokenResponse`, `getDevice`,
            `listMyDevices` for its own entry); never shown to other devices or owners (08, 5.2).
        revoke_reason (DeviceRevokeReason | Unset):
        approved_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        revoked_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        last_seen_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        push_registrations (list[PushRegistration] | Unset): Every push registration of the device, active and disabled,
            ascending by `id`, never with a token (13, 13). Absent when the device has none.
    """

    id: UUID
    user_id: UUID
    name: str
    platform: DevicePlatform
    state: DeviceState
    created_at: datetime.datetime
    app_version: str | Unset = UNSET
    key_thumbprint: str | Unset = UNSET
    step_up_key_thumbprint: str | Unset = UNSET
    verification_code: str | Unset = UNSET
    revoke_reason: DeviceRevokeReason | Unset = UNSET
    approved_at: datetime.datetime | Unset = UNSET
    revoked_at: datetime.datetime | Unset = UNSET
    last_seen_at: datetime.datetime | Unset = UNSET
    push_registrations: list[PushRegistration] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        user_id = str(self.user_id)

        name = self.name

        platform = self.platform.value

        state = self.state.value

        created_at = self.created_at.isoformat()

        app_version = self.app_version

        key_thumbprint = self.key_thumbprint

        step_up_key_thumbprint = self.step_up_key_thumbprint

        verification_code = self.verification_code

        revoke_reason: str | Unset = UNSET
        if not isinstance(self.revoke_reason, Unset):
            revoke_reason = self.revoke_reason.value

        approved_at: str | Unset = UNSET
        if not isinstance(self.approved_at, Unset):
            approved_at = self.approved_at.isoformat()

        revoked_at: str | Unset = UNSET
        if not isinstance(self.revoked_at, Unset):
            revoked_at = self.revoked_at.isoformat()

        last_seen_at: str | Unset = UNSET
        if not isinstance(self.last_seen_at, Unset):
            last_seen_at = self.last_seen_at.isoformat()

        push_registrations: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.push_registrations, Unset):
            push_registrations = []
            for push_registrations_item_data in self.push_registrations:
                push_registrations_item = push_registrations_item_data.to_dict()
                push_registrations.append(push_registrations_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "user_id": user_id,
                "name": name,
                "platform": platform,
                "state": state,
                "created_at": created_at,
            }
        )
        if app_version is not UNSET:
            field_dict["app_version"] = app_version
        if key_thumbprint is not UNSET:
            field_dict["key_thumbprint"] = key_thumbprint
        if step_up_key_thumbprint is not UNSET:
            field_dict["step_up_key_thumbprint"] = step_up_key_thumbprint
        if verification_code is not UNSET:
            field_dict["verification_code"] = verification_code
        if revoke_reason is not UNSET:
            field_dict["revoke_reason"] = revoke_reason
        if approved_at is not UNSET:
            field_dict["approved_at"] = approved_at
        if revoked_at is not UNSET:
            field_dict["revoked_at"] = revoked_at
        if last_seen_at is not UNSET:
            field_dict["last_seen_at"] = last_seen_at
        if push_registrations is not UNSET:
            field_dict["push_registrations"] = push_registrations

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.push_registration import PushRegistration

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        user_id = UUID(d.pop("user_id"))

        name = d.pop("name")

        platform = DevicePlatform(d.pop("platform"))

        state = DeviceState(d.pop("state"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        app_version = d.pop("app_version", UNSET)

        key_thumbprint = d.pop("key_thumbprint", UNSET)

        step_up_key_thumbprint = d.pop("step_up_key_thumbprint", UNSET)

        verification_code = d.pop("verification_code", UNSET)

        _revoke_reason = d.pop("revoke_reason", UNSET)
        revoke_reason: DeviceRevokeReason | Unset
        if isinstance(_revoke_reason, Unset):
            revoke_reason = UNSET
        else:
            revoke_reason = DeviceRevokeReason(_revoke_reason)

        _approved_at = d.pop("approved_at", UNSET)
        approved_at: datetime.datetime | Unset
        if isinstance(_approved_at, Unset):
            approved_at = UNSET
        else:
            approved_at = datetime.datetime.fromisoformat(_approved_at)

        _revoked_at = d.pop("revoked_at", UNSET)
        revoked_at: datetime.datetime | Unset
        if isinstance(_revoked_at, Unset):
            revoked_at = UNSET
        else:
            revoked_at = datetime.datetime.fromisoformat(_revoked_at)

        _last_seen_at = d.pop("last_seen_at", UNSET)
        last_seen_at: datetime.datetime | Unset
        if isinstance(_last_seen_at, Unset):
            last_seen_at = UNSET
        else:
            last_seen_at = datetime.datetime.fromisoformat(_last_seen_at)

        _push_registrations = d.pop("push_registrations", UNSET)
        push_registrations: list[PushRegistration] | Unset = UNSET
        if _push_registrations is not UNSET:
            push_registrations = []
            for push_registrations_item_data in _push_registrations:
                push_registrations_item = PushRegistration.from_dict(
                    push_registrations_item_data
                )

                push_registrations.append(push_registrations_item)

        device = cls(
            id=id,
            user_id=user_id,
            name=name,
            platform=platform,
            state=state,
            created_at=created_at,
            app_version=app_version,
            key_thumbprint=key_thumbprint,
            step_up_key_thumbprint=step_up_key_thumbprint,
            verification_code=verification_code,
            revoke_reason=revoke_reason,
            approved_at=approved_at,
            revoked_at=revoked_at,
            last_seen_at=last_seen_at,
            push_registrations=push_registrations,
        )

        device.additional_properties = d
        return device

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
