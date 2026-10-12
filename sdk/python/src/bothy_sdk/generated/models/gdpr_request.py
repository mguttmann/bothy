from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.gdpr_request_kind import GdprRequestKind
from ..models.gdpr_request_state import GdprRequestState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="GdprRequest")


@_attrs_define
class GdprRequest:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (GdprRequestKind):
        state (GdprRequestState):
        requested_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (str):
        due_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        approved_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        result_expires_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        progress (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
        error (str | Unset):
        completed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    subject_user_id: UUID
    kind: GdprRequestKind
    state: GdprRequestState
    requested_by: UUID
    reason: str
    due_at: datetime.datetime
    created_at: datetime.datetime
    approved_by: UUID | Unset = UNSET
    result_expires_at: datetime.datetime | Unset = UNSET
    progress: JsonObject | Unset = UNSET
    error: str | Unset = UNSET
    completed_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        subject_user_id = str(self.subject_user_id)

        kind = self.kind.value

        state = self.state.value

        requested_by = str(self.requested_by)

        reason = self.reason

        due_at = self.due_at.isoformat()

        created_at = self.created_at.isoformat()

        approved_by: str | Unset = UNSET
        if not isinstance(self.approved_by, Unset):
            approved_by = str(self.approved_by)

        result_expires_at: str | Unset = UNSET
        if not isinstance(self.result_expires_at, Unset):
            result_expires_at = self.result_expires_at.isoformat()

        progress: dict[str, Any] | Unset = UNSET
        if not isinstance(self.progress, Unset):
            progress = self.progress.to_dict()

        error = self.error

        completed_at: str | Unset = UNSET
        if not isinstance(self.completed_at, Unset):
            completed_at = self.completed_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "subject_user_id": subject_user_id,
                "kind": kind,
                "state": state,
                "requested_by": requested_by,
                "reason": reason,
                "due_at": due_at,
                "created_at": created_at,
            }
        )
        if approved_by is not UNSET:
            field_dict["approved_by"] = approved_by
        if result_expires_at is not UNSET:
            field_dict["result_expires_at"] = result_expires_at
        if progress is not UNSET:
            field_dict["progress"] = progress
        if error is not UNSET:
            field_dict["error"] = error
        if completed_at is not UNSET:
            field_dict["completed_at"] = completed_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        subject_user_id = UUID(d.pop("subject_user_id"))

        kind = GdprRequestKind(d.pop("kind"))

        state = GdprRequestState(d.pop("state"))

        requested_by = UUID(d.pop("requested_by"))

        reason = d.pop("reason")

        due_at = datetime.datetime.fromisoformat(d.pop("due_at"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _approved_by = d.pop("approved_by", UNSET)
        approved_by: UUID | Unset
        if isinstance(_approved_by, Unset):
            approved_by = UNSET
        else:
            approved_by = UUID(_approved_by)

        _result_expires_at = d.pop("result_expires_at", UNSET)
        result_expires_at: datetime.datetime | Unset
        if isinstance(_result_expires_at, Unset):
            result_expires_at = UNSET
        else:
            result_expires_at = datetime.datetime.fromisoformat(_result_expires_at)

        _progress = d.pop("progress", UNSET)
        progress: JsonObject | Unset
        if isinstance(_progress, Unset):
            progress = UNSET
        else:
            progress = JsonObject.from_dict(_progress)

        error = d.pop("error", UNSET)

        _completed_at = d.pop("completed_at", UNSET)
        completed_at: datetime.datetime | Unset
        if isinstance(_completed_at, Unset):
            completed_at = UNSET
        else:
            completed_at = datetime.datetime.fromisoformat(_completed_at)

        gdpr_request = cls(
            id=id,
            subject_user_id=subject_user_id,
            kind=kind,
            state=state,
            requested_by=requested_by,
            reason=reason,
            due_at=due_at,
            created_at=created_at,
            approved_by=approved_by,
            result_expires_at=result_expires_at,
            progress=progress,
            error=error,
            completed_at=completed_at,
        )

        gdpr_request.additional_properties = d
        return gdpr_request

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
