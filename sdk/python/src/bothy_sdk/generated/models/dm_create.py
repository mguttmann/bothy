from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="DmCreate")


@_attrs_define
class DmCreate:
    """
    Attributes:
        participant_ids (list[UUID]): Other people in the conversation; the caller is added.
    """

    participant_ids: list[UUID]

    def to_dict(self) -> dict[str, Any]:
        participant_ids = []
        for participant_ids_item_data in self.participant_ids:
            participant_ids_item = str(participant_ids_item_data)
            participant_ids.append(participant_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "participant_ids": participant_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        participant_ids = []
        _participant_ids = d.pop("participant_ids")
        for participant_ids_item_data in _participant_ids:
            participant_ids_item = UUID(participant_ids_item_data)

            participant_ids.append(participant_ids_item)

        dm_create = cls(
            participant_ids=participant_ids,
        )

        return dm_create
