from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="MessageRevision")


@_attrs_define
class MessageRevision:
    """
    Attributes:
        revision (int):
        content_md (str):
        edited_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    revision: int
    content_md: str
    edited_by: UUID
    created_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        revision = self.revision

        content_md = self.content_md

        edited_by = str(self.edited_by)

        created_at = self.created_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "revision": revision,
                "content_md": content_md,
                "edited_by": edited_by,
                "created_at": created_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        revision = d.pop("revision")

        content_md = d.pop("content_md")

        edited_by = UUID(d.pop("edited_by"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        message_revision = cls(
            revision=revision,
            content_md=content_md,
            edited_by=edited_by,
            created_at=created_at,
        )

        message_revision.additional_properties = d
        return message_revision

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
