from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="MovePosition")


@_attrs_define
class MovePosition:
    """New position relative to siblings. Exactly one of both fields; none moves to the start.

    Attributes:
        after_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        before_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    after_id: UUID | Unset = UNSET
    before_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        after_id: str | Unset = UNSET
        if not isinstance(self.after_id, Unset):
            after_id = str(self.after_id)

        before_id: str | Unset = UNSET
        if not isinstance(self.before_id, Unset):
            before_id = str(self.before_id)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if after_id is not UNSET:
            field_dict["after_id"] = after_id
        if before_id is not UNSET:
            field_dict["before_id"] = before_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        _after_id = d.pop("after_id", UNSET)
        after_id: UUID | Unset
        if isinstance(_after_id, Unset):
            after_id = UNSET
        else:
            after_id = UUID(_after_id)

        _before_id = d.pop("before_id", UNSET)
        before_id: UUID | Unset
        if isinstance(_before_id, Unset):
            before_id = UNSET
        else:
            before_id = UUID(_before_id)

        move_position = cls(
            after_id=after_id,
            before_id=before_id,
        )

        return move_position
