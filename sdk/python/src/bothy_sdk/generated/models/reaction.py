from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="Reaction")


@_attrs_define
class Reaction:
    """
    Attributes:
        emoji (str):
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    emoji: str
    principal: PrincipalRef
    created_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        emoji = self.emoji

        principal = self.principal.to_dict()

        created_at = self.created_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "emoji": emoji,
                "principal": principal,
                "created_at": created_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        emoji = d.pop("emoji")

        principal = PrincipalRef.from_dict(d.pop("principal"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        reaction = cls(
            emoji=emoji,
            principal=principal,
            created_at=created_at,
        )

        reaction.additional_properties = d
        return reaction

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
