from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.notification_preference import NotificationPreference


T = TypeVar("T", bound="NotificationPrefPayload")


@_attrs_define
class NotificationPrefPayload:
    """
    Attributes:
        preference (NotificationPreference):
        deleted (bool | Unset):
    """

    preference: NotificationPreference
    deleted: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        preference = self.preference.to_dict()

        deleted = self.deleted

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "preference": preference,
            }
        )
        if deleted is not UNSET:
            field_dict["deleted"] = deleted

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.notification_preference import NotificationPreference

        d = dict(src_dict)
        preference = NotificationPreference.from_dict(d.pop("preference"))

        deleted = d.pop("deleted", UNSET)

        notification_pref_payload = cls(
            preference=preference,
            deleted=deleted,
        )

        notification_pref_payload.additional_properties = d
        return notification_pref_payload

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
