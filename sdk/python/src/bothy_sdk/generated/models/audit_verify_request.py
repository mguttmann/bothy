from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="AuditVerifyRequest")


@_attrs_define
class AuditVerifyRequest:
    """
    Attributes:
        from_seq (int | Unset):
        to_seq (int | Unset):
    """

    from_seq: int | Unset = UNSET
    to_seq: int | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        from_seq = self.from_seq

        to_seq = self.to_seq

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if from_seq is not UNSET:
            field_dict["from_seq"] = from_seq
        if to_seq is not UNSET:
            field_dict["to_seq"] = to_seq

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        from_seq = d.pop("from_seq", UNSET)

        to_seq = d.pop("to_seq", UNSET)

        audit_verify_request = cls(
            from_seq=from_seq,
            to_seq=to_seq,
        )

        return audit_verify_request
