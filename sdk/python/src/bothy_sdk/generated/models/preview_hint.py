from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.preview_hint_code import PreviewHintCode
from ..types import UNSET, Unset

T = TypeVar("T", bound="PreviewHint")


@_attrs_define
class PreviewHint:
    """`terminal_not_effective` (grant on a host, 07, 2.12 point 4) says that SR10 or SR12 keep the grant from taking
    effect: with `principal_id` for a user subject, with `group_id` and `count` (members in effect it does not reach,
    absent when 0) for a group subject.

        Attributes:
            code (PreviewHintCode):
            agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            principal_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            group_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            count (int | Unset):
    """

    code: PreviewHintCode
    agent_id: UUID | Unset = UNSET
    principal_id: UUID | Unset = UNSET
    group_id: UUID | Unset = UNSET
    space_id: UUID | Unset = UNSET
    channel_id: UUID | Unset = UNSET
    count: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        code = self.code.value

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        principal_id: str | Unset = UNSET
        if not isinstance(self.principal_id, Unset):
            principal_id = str(self.principal_id)

        group_id: str | Unset = UNSET
        if not isinstance(self.group_id, Unset):
            group_id = str(self.group_id)

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        channel_id: str | Unset = UNSET
        if not isinstance(self.channel_id, Unset):
            channel_id = str(self.channel_id)

        count = self.count

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "code": code,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if principal_id is not UNSET:
            field_dict["principal_id"] = principal_id
        if group_id is not UNSET:
            field_dict["group_id"] = group_id
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if channel_id is not UNSET:
            field_dict["channel_id"] = channel_id
        if count is not UNSET:
            field_dict["count"] = count

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        code = PreviewHintCode(d.pop("code"))

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _principal_id = d.pop("principal_id", UNSET)
        principal_id: UUID | Unset
        if isinstance(_principal_id, Unset):
            principal_id = UNSET
        else:
            principal_id = UUID(_principal_id)

        _group_id = d.pop("group_id", UNSET)
        group_id: UUID | Unset
        if isinstance(_group_id, Unset):
            group_id = UNSET
        else:
            group_id = UUID(_group_id)

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        _channel_id = d.pop("channel_id", UNSET)
        channel_id: UUID | Unset
        if isinstance(_channel_id, Unset):
            channel_id = UNSET
        else:
            channel_id = UUID(_channel_id)

        count = d.pop("count", UNSET)

        preview_hint = cls(
            code=code,
            agent_id=agent_id,
            principal_id=principal_id,
            group_id=group_id,
            space_id=space_id,
            channel_id=channel_id,
            count=count,
        )

        preview_hint.additional_properties = d
        return preview_hint

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
