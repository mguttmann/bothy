from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.group_changed_payload_change import GroupChangedPayloadChange
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.group import Group


T = TypeVar("T", bound="GroupChangedPayload")


@_attrs_define
class GroupChangedPayload:
    """
    Attributes:
        group (Group):
        change (GroupChangedPayloadChange): `member_expired`: a membership ended by expiry, reported once by the job
            `authz.expiry_sweep`.
        user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    group: Group
    change: GroupChangedPayloadChange
    user_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        group = self.group.to_dict()

        change = self.change.value

        user_id: str | Unset = UNSET
        if not isinstance(self.user_id, Unset):
            user_id = str(self.user_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "group": group,
                "change": change,
            }
        )
        if user_id is not UNSET:
            field_dict["user_id"] = user_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.group import Group

        d = dict(src_dict)
        group = Group.from_dict(d.pop("group"))

        change = GroupChangedPayloadChange(d.pop("change"))

        _user_id = d.pop("user_id", UNSET)
        user_id: UUID | Unset
        if isinstance(_user_id, Unset):
            user_id = UNSET
        else:
            user_id = UUID(_user_id)

        group_changed_payload = cls(
            group=group,
            change=change,
            user_id=user_id,
        )

        group_changed_payload.additional_properties = d
        return group_changed_payload

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
