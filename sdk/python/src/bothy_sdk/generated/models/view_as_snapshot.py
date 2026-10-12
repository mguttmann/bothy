from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.channel_view import ChannelView
    from ..models.principal_ref import PrincipalRef
    from ..models.space_view import SpaceView


T = TypeVar("T", bound="ViewAsSnapshot")


@_attrs_define
class ViewAsSnapshot:
    """Read-only view of what a person can see, for owners (07, 9.2). Direct messages never appear.

    Attributes:
        user (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        spaces (list[SpaceView]):
        channels (list[ChannelView]):
    """

    user: PrincipalRef
    expires_at: datetime.datetime
    spaces: list[SpaceView]
    channels: list[ChannelView]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user = self.user.to_dict()

        expires_at = self.expires_at.isoformat()

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
                "user": user,
                "expires_at": expires_at,
                "spaces": spaces,
                "channels": channels,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.channel_view import ChannelView
        from ..models.principal_ref import PrincipalRef
        from ..models.space_view import SpaceView

        d = dict(src_dict)
        user = PrincipalRef.from_dict(d.pop("user"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        spaces = []
        _spaces = d.pop("spaces")
        for spaces_item_data in _spaces:
            spaces_item = SpaceView.from_dict(spaces_item_data)

            spaces.append(spaces_item)

        channels = []
        _channels = d.pop("channels")
        for channels_item_data in _channels:
            channels_item = ChannelView.from_dict(channels_item_data)

            channels.append(channels_item)

        view_as_snapshot = cls(
            user=user,
            expires_at=expires_at,
            spaces=spaces,
            channels=channels,
        )

        view_as_snapshot.additional_properties = d
        return view_as_snapshot

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
