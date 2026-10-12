from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.audit_verify_result_failure_reason import AuditVerifyResultFailureReason
from ..types import UNSET, Unset

T = TypeVar("T", bound="AuditVerifyResult")


@_attrs_define
class AuditVerifyResult:
    """
    Attributes:
        ok (bool):
        checked_from_seq (int):
        checked_to_seq (int):
        checkpoints_compared (int):
        warnings (list[str]): Known value `checkpoint_sink_not_configured`.
        first_broken_seq (int | Unset):
        failure_reason (AuditVerifyResultFailureReason | Unset):
        last_checkpoint_seq (int | Unset):
    """

    ok: bool
    checked_from_seq: int
    checked_to_seq: int
    checkpoints_compared: int
    warnings: list[str]
    first_broken_seq: int | Unset = UNSET
    failure_reason: AuditVerifyResultFailureReason | Unset = UNSET
    last_checkpoint_seq: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        ok = self.ok

        checked_from_seq = self.checked_from_seq

        checked_to_seq = self.checked_to_seq

        checkpoints_compared = self.checkpoints_compared

        warnings = self.warnings

        first_broken_seq = self.first_broken_seq

        failure_reason: str | Unset = UNSET
        if not isinstance(self.failure_reason, Unset):
            failure_reason = self.failure_reason.value

        last_checkpoint_seq = self.last_checkpoint_seq

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "ok": ok,
                "checked_from_seq": checked_from_seq,
                "checked_to_seq": checked_to_seq,
                "checkpoints_compared": checkpoints_compared,
                "warnings": warnings,
            }
        )
        if first_broken_seq is not UNSET:
            field_dict["first_broken_seq"] = first_broken_seq
        if failure_reason is not UNSET:
            field_dict["failure_reason"] = failure_reason
        if last_checkpoint_seq is not UNSET:
            field_dict["last_checkpoint_seq"] = last_checkpoint_seq

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        ok = d.pop("ok")

        checked_from_seq = d.pop("checked_from_seq")

        checked_to_seq = d.pop("checked_to_seq")

        checkpoints_compared = d.pop("checkpoints_compared")

        warnings = cast(list[str], d.pop("warnings"))

        first_broken_seq = d.pop("first_broken_seq", UNSET)

        _failure_reason = d.pop("failure_reason", UNSET)
        failure_reason: AuditVerifyResultFailureReason | Unset
        if isinstance(_failure_reason, Unset):
            failure_reason = UNSET
        else:
            failure_reason = AuditVerifyResultFailureReason(_failure_reason)

        last_checkpoint_seq = d.pop("last_checkpoint_seq", UNSET)

        audit_verify_result = cls(
            ok=ok,
            checked_from_seq=checked_from_seq,
            checked_to_seq=checked_to_seq,
            checkpoints_compared=checkpoints_compared,
            warnings=warnings,
            first_broken_seq=first_broken_seq,
            failure_reason=failure_reason,
            last_checkpoint_seq=last_checkpoint_seq,
        )

        audit_verify_result.additional_properties = d
        return audit_verify_result

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
