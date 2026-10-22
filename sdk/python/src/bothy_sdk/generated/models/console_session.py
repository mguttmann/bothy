from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.console_session_step_up_method import ConsoleSessionStepUpMethod
from ..models.console_session_step_up_methods_available_item import (
    ConsoleSessionStepUpMethodsAvailableItem,
)
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.console_session_view_as import ConsoleSessionViewAs


T = TypeVar("T", bound="ConsoleSession")


@_attrs_define
class ConsoleSession:
    """
    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        display_name (str):
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        idle_timeout_seconds (int):
        step_up_methods_available (list[ConsoleSessionStepUpMethodsAvailableItem]):
        step_up_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        step_up_method (ConsoleSessionStepUpMethod | Unset):
        step_up_valid_until (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        view_as (ConsoleSessionViewAs | Unset):
    """

    user_id: UUID
    display_name: str
    expires_at: datetime.datetime
    idle_timeout_seconds: int
    step_up_methods_available: list[ConsoleSessionStepUpMethodsAvailableItem]
    step_up_at: datetime.datetime | Unset = UNSET
    step_up_method: ConsoleSessionStepUpMethod | Unset = UNSET
    step_up_valid_until: datetime.datetime | Unset = UNSET
    view_as: ConsoleSessionViewAs | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        display_name = self.display_name

        expires_at = self.expires_at.isoformat()

        idle_timeout_seconds = self.idle_timeout_seconds

        step_up_methods_available = []
        for step_up_methods_available_item_data in self.step_up_methods_available:
            step_up_methods_available_item = step_up_methods_available_item_data.value
            step_up_methods_available.append(step_up_methods_available_item)

        step_up_at: str | Unset = UNSET
        if not isinstance(self.step_up_at, Unset):
            step_up_at = self.step_up_at.isoformat()

        step_up_method: str | Unset = UNSET
        if not isinstance(self.step_up_method, Unset):
            step_up_method = self.step_up_method.value

        step_up_valid_until: str | Unset = UNSET
        if not isinstance(self.step_up_valid_until, Unset):
            step_up_valid_until = self.step_up_valid_until.isoformat()

        view_as: dict[str, Any] | Unset = UNSET
        if not isinstance(self.view_as, Unset):
            view_as = self.view_as.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user_id": user_id,
                "display_name": display_name,
                "expires_at": expires_at,
                "idle_timeout_seconds": idle_timeout_seconds,
                "step_up_methods_available": step_up_methods_available,
            }
        )
        if step_up_at is not UNSET:
            field_dict["step_up_at"] = step_up_at
        if step_up_method is not UNSET:
            field_dict["step_up_method"] = step_up_method
        if step_up_valid_until is not UNSET:
            field_dict["step_up_valid_until"] = step_up_valid_until
        if view_as is not UNSET:
            field_dict["view_as"] = view_as

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.console_session_view_as import ConsoleSessionViewAs

        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        display_name = d.pop("display_name")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        idle_timeout_seconds = d.pop("idle_timeout_seconds")

        step_up_methods_available = []
        _step_up_methods_available = d.pop("step_up_methods_available")
        for step_up_methods_available_item_data in _step_up_methods_available:
            step_up_methods_available_item = ConsoleSessionStepUpMethodsAvailableItem(
                step_up_methods_available_item_data
            )

            step_up_methods_available.append(step_up_methods_available_item)

        _step_up_at = d.pop("step_up_at", UNSET)
        step_up_at: datetime.datetime | Unset
        if isinstance(_step_up_at, Unset):
            step_up_at = UNSET
        else:
            step_up_at = datetime.datetime.fromisoformat(_step_up_at)

        _step_up_method = d.pop("step_up_method", UNSET)
        step_up_method: ConsoleSessionStepUpMethod | Unset
        if isinstance(_step_up_method, Unset):
            step_up_method = UNSET
        else:
            step_up_method = ConsoleSessionStepUpMethod(_step_up_method)

        _step_up_valid_until = d.pop("step_up_valid_until", UNSET)
        step_up_valid_until: datetime.datetime | Unset
        if isinstance(_step_up_valid_until, Unset):
            step_up_valid_until = UNSET
        else:
            step_up_valid_until = datetime.datetime.fromisoformat(_step_up_valid_until)

        _view_as = d.pop("view_as", UNSET)
        view_as: ConsoleSessionViewAs | Unset
        if isinstance(_view_as, Unset):
            view_as = UNSET
        else:
            view_as = ConsoleSessionViewAs.from_dict(_view_as)

        console_session = cls(
            user_id=user_id,
            display_name=display_name,
            expires_at=expires_at,
            idle_timeout_seconds=idle_timeout_seconds,
            step_up_methods_available=step_up_methods_available,
            step_up_at=step_up_at,
            step_up_method=step_up_method,
            step_up_valid_until=step_up_valid_until,
            view_as=view_as,
        )

        console_session.additional_properties = d
        return console_session

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
