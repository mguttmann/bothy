from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.access_target_kind import AccessTargetKind
from ..models.capability import Capability
from ..models.level import Level
from ..types import UNSET, Unset

T = TypeVar("T", bound="AccessTarget")


@_attrs_define
class AccessTarget:
    """
    Attributes:
        kind (AccessTargetKind): `host` appears only in `access.changed` and `access.expiring` (terminal right, no
            level).
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        level (Level | Unset): Grant level. Ordered `read` < `write` < `trigger`.
        previous_level (Level | Unset): Grant level. Ordered `read` < `write` < `trigger`.
        capabilities (list[Capability] | Unset):
    """

    kind: AccessTargetKind
    id: UUID
    level: Level | Unset = UNSET
    previous_level: Level | Unset = UNSET
    capabilities: list[Capability] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        id = str(self.id)

        level: str | Unset = UNSET
        if not isinstance(self.level, Unset):
            level = self.level.value

        previous_level: str | Unset = UNSET
        if not isinstance(self.previous_level, Unset):
            previous_level = self.previous_level.value

        capabilities: list[str] | Unset = UNSET
        if not isinstance(self.capabilities, Unset):
            capabilities = []
            for capabilities_item_data in self.capabilities:
                capabilities_item = capabilities_item_data.value
                capabilities.append(capabilities_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
                "id": id,
            }
        )
        if level is not UNSET:
            field_dict["level"] = level
        if previous_level is not UNSET:
            field_dict["previous_level"] = previous_level
        if capabilities is not UNSET:
            field_dict["capabilities"] = capabilities

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = AccessTargetKind(d.pop("kind"))

        id = UUID(d.pop("id"))

        _level = d.pop("level", UNSET)
        level: Level | Unset
        if isinstance(_level, Unset):
            level = UNSET
        else:
            level = Level(_level)

        _previous_level = d.pop("previous_level", UNSET)
        previous_level: Level | Unset
        if isinstance(_previous_level, Unset):
            previous_level = UNSET
        else:
            previous_level = Level(_previous_level)

        _capabilities = d.pop("capabilities", UNSET)
        capabilities: list[Capability] | Unset = UNSET
        if _capabilities is not UNSET:
            capabilities = []
            for capabilities_item_data in _capabilities:
                capabilities_item = Capability(capabilities_item_data)

                capabilities.append(capabilities_item)

        access_target = cls(
            kind=kind,
            id=id,
            level=level,
            previous_level=previous_level,
            capabilities=capabilities,
        )

        access_target.additional_properties = d
        return access_target

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
