from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.operator_mode import OperatorMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="ChannelOperator")


@_attrs_define
class ChannelOperator:
    """
    Attributes:
        mode (OperatorMode): `inherit` takes the default operator of the space (channels only).
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        effective_agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    mode: OperatorMode
    agent_id: UUID | Unset = UNSET
    effective_agent_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        mode = self.mode.value

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        effective_agent_id: str | Unset = UNSET
        if not isinstance(self.effective_agent_id, Unset):
            effective_agent_id = str(self.effective_agent_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "mode": mode,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if effective_agent_id is not UNSET:
            field_dict["effective_agent_id"] = effective_agent_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        mode = OperatorMode(d.pop("mode"))

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _effective_agent_id = d.pop("effective_agent_id", UNSET)
        effective_agent_id: UUID | Unset
        if isinstance(_effective_agent_id, Unset):
            effective_agent_id = UNSET
        else:
            effective_agent_id = UUID(_effective_agent_id)

        channel_operator = cls(
            mode=mode,
            agent_id=agent_id,
            effective_agent_id=effective_agent_id,
        )

        channel_operator.additional_properties = d
        return channel_operator

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
