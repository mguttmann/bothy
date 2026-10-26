from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="AppLockPolicy")


@_attrs_define
class AppLockPolicy:
    """The app lock policy of the phone apps (08 9): the server cannot check the lock, so it only delivers the two values
    the app enforces. Both come from the process configuration, not from an instance setting.

        Attributes:
            required (bool): Whether the owner requires the app lock on every device (`security.app_lock_required`); the app
                then shows the switch on and disabled, and a device without a device credential shows no content.
            after_ms (int): Time without use after which an app with the lock on asks for the system authentication again
                (`client.app_lock_after`), in milliseconds.
    """

    required: bool
    after_ms: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        required = self.required

        after_ms = self.after_ms

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "required": required,
                "after_ms": after_ms,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        required = d.pop("required")

        after_ms = d.pop("after_ms")

        app_lock_policy = cls(
            required=required,
            after_ms=after_ms,
        )

        app_lock_policy.additional_properties = d
        return app_lock_policy

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
