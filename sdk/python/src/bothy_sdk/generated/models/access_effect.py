from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.level_or_none import LevelOrNone
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="AccessEffect")


@_attrs_define
class AccessEffect:
    """
    Attributes:
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        target_kind (TargetKind): `host` is a connected host for the terminal; it has no level and only the capability
            `terminal` (24, 8.1).
        before_level (LevelOrNone): Level including `none` for no access.
        after_level (LevelOrNone): Level including `none` for no access.
        capabilities_added (list[Capability]):
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    principal: PrincipalRef
    target_kind: TargetKind
    before_level: LevelOrNone
    after_level: LevelOrNone
    capabilities_added: list[Capability]
    target_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        principal = self.principal.to_dict()

        target_kind = self.target_kind.value

        before_level = self.before_level.value

        after_level = self.after_level.value

        capabilities_added = []
        for capabilities_added_item_data in self.capabilities_added:
            capabilities_added_item = capabilities_added_item_data.value
            capabilities_added.append(capabilities_added_item)

        target_id: str | Unset = UNSET
        if not isinstance(self.target_id, Unset):
            target_id = str(self.target_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "principal": principal,
                "target_kind": target_kind,
                "before_level": before_level,
                "after_level": after_level,
                "capabilities_added": capabilities_added,
            }
        )
        if target_id is not UNSET:
            field_dict["target_id"] = target_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        principal = PrincipalRef.from_dict(d.pop("principal"))

        target_kind = TargetKind(d.pop("target_kind"))

        before_level = LevelOrNone(d.pop("before_level"))

        after_level = LevelOrNone(d.pop("after_level"))

        capabilities_added = []
        _capabilities_added = d.pop("capabilities_added")
        for capabilities_added_item_data in _capabilities_added:
            capabilities_added_item = Capability(capabilities_added_item_data)

            capabilities_added.append(capabilities_added_item)

        _target_id = d.pop("target_id", UNSET)
        target_id: UUID | Unset
        if isinstance(_target_id, Unset):
            target_id = UNSET
        else:
            target_id = UUID(_target_id)

        access_effect = cls(
            principal=principal,
            target_kind=target_kind,
            before_level=before_level,
            after_level=after_level,
            capabilities_added=capabilities_added,
            target_id=target_id,
        )

        access_effect.additional_properties = d
        return access_effect

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
