from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.level import Level

if TYPE_CHECKING:
    from ..models.channel import Channel


T = TypeVar("T", bound="ChannelView")


@_attrs_define
class ChannelView:
    """
    Attributes:
        channel (Channel):
        level (Level): Grant level. Ordered `read` < `write` < `trigger`.
        capabilities (list[Capability]):
    """

    channel: Channel
    level: Level
    capabilities: list[Capability]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel = self.channel.to_dict()

        level = self.level.value

        capabilities = []
        for capabilities_item_data in self.capabilities:
            capabilities_item = capabilities_item_data.value
            capabilities.append(capabilities_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel": channel,
                "level": level,
                "capabilities": capabilities,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.channel import Channel

        d = dict(src_dict)
        channel = Channel.from_dict(d.pop("channel"))

        level = Level(d.pop("level"))

        capabilities = []
        _capabilities = d.pop("capabilities")
        for capabilities_item_data in _capabilities:
            capabilities_item = Capability(capabilities_item_data)

            capabilities.append(capabilities_item)

        channel_view = cls(
            channel=channel,
            level=level,
            capabilities=capabilities,
        )

        channel_view.additional_properties = d
        return channel_view

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
