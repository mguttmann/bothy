from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.gdpr_request_kind import GdprRequestKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="GdprRequestCreate")


@_attrs_define
class GdprRequestCreate:
    """
    Attributes:
        kind (GdprRequestKind):
        reason (str):
        subject_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    kind: GdprRequestKind
    reason: str
    subject_user_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        reason = self.reason

        subject_user_id: str | Unset = UNSET
        if not isinstance(self.subject_user_id, Unset):
            subject_user_id = str(self.subject_user_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "reason": reason,
            }
        )
        if subject_user_id is not UNSET:
            field_dict["subject_user_id"] = subject_user_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = GdprRequestKind(d.pop("kind"))

        reason = d.pop("reason")

        _subject_user_id = d.pop("subject_user_id", UNSET)
        subject_user_id: UUID | Unset
        if isinstance(_subject_user_id, Unset):
            subject_user_id = UNSET
        else:
            subject_user_id = UUID(_subject_user_id)

        gdpr_request_create = cls(
            kind=kind,
            reason=reason,
            subject_user_id=subject_user_id,
        )

        return gdpr_request_create
