from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.level_or_none import LevelOrNone
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.access_source import AccessSource
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="AccessEntry")


@_attrs_define
class AccessEntry:
    """
    Attributes:
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        level (LevelOrNone): Level including `none` for no access.
        capabilities (list[Capability]):
        sources (list[AccessSource]):
        inactive (bool | Unset): True when the principal is not `active`; the level is what its grants would give (07,
            9.1).
    """

    principal: PrincipalRef
    level: LevelOrNone
    capabilities: list[Capability]
    sources: list[AccessSource]
    inactive: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        principal = self.principal.to_dict()

        level = self.level.value

        capabilities = []
        for capabilities_item_data in self.capabilities:
            capabilities_item = capabilities_item_data.value
            capabilities.append(capabilities_item)

        sources = []
        for sources_item_data in self.sources:
            sources_item = sources_item_data.to_dict()
            sources.append(sources_item)

        inactive = self.inactive

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "principal": principal,
                "level": level,
                "capabilities": capabilities,
                "sources": sources,
            }
        )
        if inactive is not UNSET:
            field_dict["inactive"] = inactive

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_source import AccessSource
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        principal = PrincipalRef.from_dict(d.pop("principal"))

        level = LevelOrNone(d.pop("level"))

        capabilities = []
        _capabilities = d.pop("capabilities")
        for capabilities_item_data in _capabilities:
            capabilities_item = Capability(capabilities_item_data)

            capabilities.append(capabilities_item)

        sources = []
        _sources = d.pop("sources")
        for sources_item_data in _sources:
            sources_item = AccessSource.from_dict(sources_item_data)

            sources.append(sources_item)

        inactive = d.pop("inactive", UNSET)

        access_entry = cls(
            principal=principal,
            level=level,
            capabilities=capabilities,
            sources=sources,
            inactive=inactive,
        )

        access_entry.additional_properties = d
        return access_entry

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
