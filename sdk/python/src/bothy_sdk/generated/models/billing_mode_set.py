from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="BillingModeSet")


@_attrs_define
class BillingModeSet:
    """
    Attributes:
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        reason (str):
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    billing_mode: BillingMode
    reason: str
    credential_holder_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        billing_mode = self.billing_mode.value

        reason = self.reason

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "billing_mode": billing_mode,
                "reason": reason,
            }
        )
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        billing_mode = BillingMode(d.pop("billing_mode"))

        reason = d.pop("reason")

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        billing_mode_set = cls(
            billing_mode=billing_mode,
            reason=reason,
            credential_holder_id=credential_holder_id,
        )

        return billing_mode_set
