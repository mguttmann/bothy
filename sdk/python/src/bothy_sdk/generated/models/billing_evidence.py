from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.billing_evidence_key_type import BillingEvidenceKeyType
from ..types import UNSET, Unset

T = TypeVar("T", bound="BillingEvidence")


@_attrs_define
class BillingEvidence:
    """How the runtime proves its actual billing mode. Never a key value or prefix (schema `billing_evidence.v1`).

    Attributes:
        key_type (BillingEvidenceKeyType):
        provider (str | Unset):
        method (str | Unset):
    """

    key_type: BillingEvidenceKeyType
    provider: str | Unset = UNSET
    method: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        key_type = self.key_type.value

        provider = self.provider

        method = self.method

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "key_type": key_type,
            }
        )
        if provider is not UNSET:
            field_dict["provider"] = provider
        if method is not UNSET:
            field_dict["method"] = method

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        key_type = BillingEvidenceKeyType(d.pop("key_type"))

        provider = d.pop("provider", UNSET)

        method = d.pop("method", UNSET)

        billing_evidence = cls(
            key_type=key_type,
            provider=provider,
            method=method,
        )

        return billing_evidence
