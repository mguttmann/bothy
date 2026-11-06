from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="HostSet")


@_attrs_define
class HostSet:
    """
    Attributes:
        hosted_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (str):
    """

    hosted_by: UUID
    reason: str

    def to_dict(self) -> dict[str, Any]:
        hosted_by = str(self.hosted_by)

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "hosted_by": hosted_by,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        hosted_by = UUID(d.pop("hosted_by"))

        reason = d.pop("reason")

        host_set = cls(
            hosted_by=hosted_by,
            reason=reason,
        )

        return host_set
