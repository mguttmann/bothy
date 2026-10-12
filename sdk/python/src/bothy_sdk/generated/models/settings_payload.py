from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.user_setting import UserSetting


T = TypeVar("T", bound="SettingsPayload")


@_attrs_define
class SettingsPayload:
    """
    Attributes:
        setting (UserSetting):
        deleted (bool | Unset):
    """

    setting: UserSetting
    deleted: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        setting = self.setting.to_dict()

        deleted = self.deleted

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "setting": setting,
            }
        )
        if deleted is not UNSET:
            field_dict["deleted"] = deleted

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.user_setting import UserSetting

        d = dict(src_dict)
        setting = UserSetting.from_dict(d.pop("setting"))

        deleted = d.pop("deleted", UNSET)

        settings_payload = cls(
            setting=setting,
            deleted=deleted,
        )

        settings_payload.additional_properties = d
        return settings_payload

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
