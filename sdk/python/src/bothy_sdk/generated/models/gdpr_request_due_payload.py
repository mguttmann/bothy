from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.gdpr_request_kind import GdprRequestKind

T = TypeVar("T", bound="GdprRequestDuePayload")


@_attrs_define
class GdprRequestDuePayload:
    """An open privacy request reaches the reminder lead before its statutory deadline (R-AZ-35). Carries only the ID, the
    kind and the deadline.

        Attributes:
            request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            kind (GdprRequestKind):
            due_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    request_id: UUID
    kind: GdprRequestKind
    due_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        request_id = str(self.request_id)

        kind = self.kind.value

        due_at = self.due_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "request_id": request_id,
                "kind": kind,
                "due_at": due_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        request_id = UUID(d.pop("request_id"))

        kind = GdprRequestKind(d.pop("kind"))

        due_at = datetime.datetime.fromisoformat(d.pop("due_at"))

        gdpr_request_due_payload = cls(
            request_id=request_id,
            kind=kind,
            due_at=due_at,
        )

        gdpr_request_due_payload.additional_properties = d
        return gdpr_request_due_payload

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
