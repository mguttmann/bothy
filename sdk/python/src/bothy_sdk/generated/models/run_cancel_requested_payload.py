from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_cancel_requested_payload_reason import RunCancelRequestedPayloadReason
from ..types import UNSET, Unset

T = TypeVar("T", bound="RunCancelRequestedPayload")


@_attrs_define
class RunCancelRequestedPayload:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (RunCancelRequestedPayloadReason):
        requested_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    run_id: UUID
    thread_id: UUID
    reason: RunCancelRequestedPayloadReason
    requested_by: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        thread_id = str(self.thread_id)

        reason = self.reason.value

        requested_by: str | Unset = UNSET
        if not isinstance(self.requested_by, Unset):
            requested_by = str(self.requested_by)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "thread_id": thread_id,
                "reason": reason,
            }
        )
        if requested_by is not UNSET:
            field_dict["requested_by"] = requested_by

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        thread_id = UUID(d.pop("thread_id"))

        reason = RunCancelRequestedPayloadReason(d.pop("reason"))

        _requested_by = d.pop("requested_by", UNSET)
        requested_by: UUID | Unset
        if isinstance(_requested_by, Unset):
            requested_by = UNSET
        else:
            requested_by = UUID(_requested_by)

        run_cancel_requested_payload = cls(
            run_id=run_id,
            thread_id=thread_id,
            reason=reason,
            requested_by=requested_by,
        )

        run_cancel_requested_payload.additional_properties = d
        return run_cancel_requested_payload

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
