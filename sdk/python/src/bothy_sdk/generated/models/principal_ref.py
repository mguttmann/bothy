from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.principal_kind import PrincipalKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="PrincipalRef")


@_attrs_define
class PrincipalRef:
    """Minimal public view of a principal, embedded in other objects.

    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (PrincipalKind):
        handle (str):
        display_name (str):
        avatar_file_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    id: UUID
    kind: PrincipalKind
    handle: str
    display_name: str
    avatar_file_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        handle = self.handle

        display_name = self.display_name

        avatar_file_id: str | Unset = UNSET
        if not isinstance(self.avatar_file_id, Unset):
            avatar_file_id = str(self.avatar_file_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "kind": kind,
                "handle": handle,
                "display_name": display_name,
            }
        )
        if avatar_file_id is not UNSET:
            field_dict["avatar_file_id"] = avatar_file_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = PrincipalKind(d.pop("kind"))

        handle = d.pop("handle")

        display_name = d.pop("display_name")

        _avatar_file_id = d.pop("avatar_file_id", UNSET)
        avatar_file_id: UUID | Unset
        if isinstance(_avatar_file_id, Unset):
            avatar_file_id = UNSET
        else:
            avatar_file_id = UUID(_avatar_file_id)

        principal_ref = cls(
            id=id,
            kind=kind,
            handle=handle,
            display_name=display_name,
            avatar_file_id=avatar_file_id,
        )

        principal_ref.additional_properties = d
        return principal_ref

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
