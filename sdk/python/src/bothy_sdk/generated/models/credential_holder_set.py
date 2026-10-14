from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="CredentialHolderSet")


@_attrs_define
class CredentialHolderSet:
    """
    Attributes:
        credential_holder_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (str):
    """

    credential_holder_id: UUID
    reason: str

    def to_dict(self) -> dict[str, Any]:
        credential_holder_id = str(self.credential_holder_id)

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "credential_holder_id": credential_holder_id,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        credential_holder_id = UUID(d.pop("credential_holder_id"))

        reason = d.pop("reason")

        credential_holder_set = cls(
            credential_holder_id=credential_holder_id,
            reason=reason,
        )

        return credential_holder_set
