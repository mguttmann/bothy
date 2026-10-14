from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="OwnerAdd")


@_attrs_define
class OwnerAdd:
    """
    Attributes:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (str):
    """

    user_id: UUID
    reason: str

    def to_dict(self) -> dict[str, Any]:
        user_id = str(self.user_id)

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "user_id": user_id,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        user_id = UUID(d.pop("user_id"))

        reason = d.pop("reason")

        owner_add = cls(
            user_id=user_id,
            reason=reason,
        )

        return owner_add
