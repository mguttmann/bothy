from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="FlagRollout")


@_attrs_define
class FlagRollout:
    """Principals and groups for which the flag has the opposite value of `enabled`.

    Attributes:
        user_ids (list[UUID] | Unset):
        group_ids (list[UUID] | Unset):
    """

    user_ids: list[UUID] | Unset = UNSET
    group_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        user_ids: list[str] | Unset = UNSET
        if not isinstance(self.user_ids, Unset):
            user_ids = []
            for user_ids_item_data in self.user_ids:
                user_ids_item = str(user_ids_item_data)
                user_ids.append(user_ids_item)

        group_ids: list[str] | Unset = UNSET
        if not isinstance(self.group_ids, Unset):
            group_ids = []
            for group_ids_item_data in self.group_ids:
                group_ids_item = str(group_ids_item_data)
                group_ids.append(group_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if user_ids is not UNSET:
            field_dict["user_ids"] = user_ids
        if group_ids is not UNSET:
            field_dict["group_ids"] = group_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _user_ids = d.pop("user_ids", UNSET)
        user_ids: list[UUID] | Unset = UNSET
        if _user_ids is not UNSET:
            user_ids = []
            for user_ids_item_data in _user_ids:
                user_ids_item = UUID(user_ids_item_data)

                user_ids.append(user_ids_item)

        _group_ids = d.pop("group_ids", UNSET)
        group_ids: list[UUID] | Unset = UNSET
        if _group_ids is not UNSET:
            group_ids = []
            for group_ids_item_data in _group_ids:
                group_ids_item = UUID(group_ids_item_data)

                group_ids.append(group_ids_item)

        flag_rollout = cls(
            user_ids=user_ids,
            group_ids=group_ids,
        )

        return flag_rollout
