from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.channel_behavior import ChannelBehavior


T = TypeVar("T", bound="AgentChannelConfig")


@_attrs_define
class AgentChannelConfig:
    """Configuration an operator agent loads for a channel it serves.

    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        behavior (ChannelBehavior): Behavior of the operator in a channel. Readable only with `manage`; never in events.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    channel_id: UUID
    behavior: ChannelBehavior
    space_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        behavior = self.behavior.to_dict()

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel_id": channel_id,
                "behavior": behavior,
            }
        )
        if space_id is not UNSET:
            field_dict["space_id"] = space_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.channel_behavior import ChannelBehavior

        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        behavior = ChannelBehavior.from_dict(d.pop("behavior"))

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        agent_channel_config = cls(
            channel_id=channel_id,
            behavior=behavior,
            space_id=space_id,
        )

        agent_channel_config.additional_properties = d
        return agent_channel_config

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
