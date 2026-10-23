from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.access_label import AccessLabel
    from ..models.channel_label import ChannelLabel
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="AccessLabels")


@_attrs_define
class AccessLabels:
    """The names of the principals, groups, spaces and channels a page of hints or expiring access names, as the
    administration lists show them; never content.

        Attributes:
            principals (list[PrincipalRef]):
            groups (list[AccessLabel]):
            spaces (list[AccessLabel]):
            channels (list[ChannelLabel]):
    """

    principals: list[PrincipalRef]
    groups: list[AccessLabel]
    spaces: list[AccessLabel]
    channels: list[ChannelLabel]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        principals = []
        for principals_item_data in self.principals:
            principals_item = principals_item_data.to_dict()
            principals.append(principals_item)

        groups = []
        for groups_item_data in self.groups:
            groups_item = groups_item_data.to_dict()
            groups.append(groups_item)

        spaces = []
        for spaces_item_data in self.spaces:
            spaces_item = spaces_item_data.to_dict()
            spaces.append(spaces_item)

        channels = []
        for channels_item_data in self.channels:
            channels_item = channels_item_data.to_dict()
            channels.append(channels_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "principals": principals,
                "groups": groups,
                "spaces": spaces,
                "channels": channels,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_label import AccessLabel
        from ..models.channel_label import ChannelLabel
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        principals = []
        _principals = d.pop("principals")
        for principals_item_data in _principals:
            principals_item = PrincipalRef.from_dict(principals_item_data)

            principals.append(principals_item)

        groups = []
        _groups = d.pop("groups")
        for groups_item_data in _groups:
            groups_item = AccessLabel.from_dict(groups_item_data)

            groups.append(groups_item)

        spaces = []
        _spaces = d.pop("spaces")
        for spaces_item_data in _spaces:
            spaces_item = AccessLabel.from_dict(spaces_item_data)

            spaces.append(spaces_item)

        channels = []
        _channels = d.pop("channels")
        for channels_item_data in _channels:
            channels_item = ChannelLabel.from_dict(channels_item_data)

            channels.append(channels_item)

        access_labels = cls(
            principals=principals,
            groups=groups,
            spaces=spaces,
            channels=channels,
        )

        access_labels.additional_properties = d
        return access_labels

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
