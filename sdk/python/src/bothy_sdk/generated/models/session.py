from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.session_auth_method import SessionAuthMethod
from ..models.session_client_component import SessionClientComponent
from ..models.session_step_up_method import SessionStepUpMethod
from ..types import UNSET, Unset

T = TypeVar("T", bound="Session")


@_attrs_define
class Session:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        device_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        auth_method (SessionAuthMethod):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        client_component (SessionClientComponent | Unset):
        step_up_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        step_up_method (SessionStepUpMethod | Unset):
        last_seen_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        current (bool | Unset): True for the session of this request. Set only in REST responses.
    """

    id: UUID
    user_id: UUID
    device_id: UUID
    auth_method: SessionAuthMethod
    created_at: datetime.datetime
    expires_at: datetime.datetime
    client_component: SessionClientComponent | Unset = UNSET
    step_up_at: datetime.datetime | Unset = UNSET
    step_up_method: SessionStepUpMethod | Unset = UNSET
    last_seen_at: datetime.datetime | Unset = UNSET
    current: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        user_id = str(self.user_id)

        device_id = str(self.device_id)

        auth_method = self.auth_method.value

        created_at = self.created_at.isoformat()

        expires_at = self.expires_at.isoformat()

        client_component: str | Unset = UNSET
        if not isinstance(self.client_component, Unset):
            client_component = self.client_component.value

        step_up_at: str | Unset = UNSET
        if not isinstance(self.step_up_at, Unset):
            step_up_at = self.step_up_at.isoformat()

        step_up_method: str | Unset = UNSET
        if not isinstance(self.step_up_method, Unset):
            step_up_method = self.step_up_method.value

        last_seen_at: str | Unset = UNSET
        if not isinstance(self.last_seen_at, Unset):
            last_seen_at = self.last_seen_at.isoformat()

        current = self.current

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "user_id": user_id,
                "device_id": device_id,
                "auth_method": auth_method,
                "created_at": created_at,
                "expires_at": expires_at,
            }
        )
        if client_component is not UNSET:
            field_dict["client_component"] = client_component
        if step_up_at is not UNSET:
            field_dict["step_up_at"] = step_up_at
        if step_up_method is not UNSET:
            field_dict["step_up_method"] = step_up_method
        if last_seen_at is not UNSET:
            field_dict["last_seen_at"] = last_seen_at
        if current is not UNSET:
            field_dict["current"] = current

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        user_id = UUID(d.pop("user_id"))

        device_id = UUID(d.pop("device_id"))

        auth_method = SessionAuthMethod(d.pop("auth_method"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _client_component = d.pop("client_component", UNSET)
        client_component: SessionClientComponent | Unset
        if isinstance(_client_component, Unset):
            client_component = UNSET
        else:
            client_component = SessionClientComponent(_client_component)

        _step_up_at = d.pop("step_up_at", UNSET)
        step_up_at: datetime.datetime | Unset
        if isinstance(_step_up_at, Unset):
            step_up_at = UNSET
        else:
            step_up_at = datetime.datetime.fromisoformat(_step_up_at)

        _step_up_method = d.pop("step_up_method", UNSET)
        step_up_method: SessionStepUpMethod | Unset
        if isinstance(_step_up_method, Unset):
            step_up_method = UNSET
        else:
            step_up_method = SessionStepUpMethod(_step_up_method)

        _last_seen_at = d.pop("last_seen_at", UNSET)
        last_seen_at: datetime.datetime | Unset
        if isinstance(_last_seen_at, Unset):
            last_seen_at = UNSET
        else:
            last_seen_at = datetime.datetime.fromisoformat(_last_seen_at)

        current = d.pop("current", UNSET)

        session = cls(
            id=id,
            user_id=user_id,
            device_id=device_id,
            auth_method=auth_method,
            created_at=created_at,
            expires_at=expires_at,
            client_component=client_component,
            step_up_at=step_up_at,
            step_up_method=step_up_method,
            last_seen_at=last_seen_at,
            current=current,
        )

        session.additional_properties = d
        return session

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
