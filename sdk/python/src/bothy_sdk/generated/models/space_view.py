from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.capability import Capability
from ..models.level import Level

if TYPE_CHECKING:
    from ..models.space import Space


T = TypeVar("T", bound="SpaceView")


@_attrs_define
class SpaceView:
    """
    Attributes:
        space (Space):
        level (Level): Grant level. Ordered `read` < `write` < `trigger`.
        capabilities (list[Capability]):
    """

    space: Space
    level: Level
    capabilities: list[Capability]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        space = self.space.to_dict()

        level = self.level.value

        capabilities = []
        for capabilities_item_data in self.capabilities:
            capabilities_item = capabilities_item_data.value
            capabilities.append(capabilities_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "space": space,
                "level": level,
                "capabilities": capabilities,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.space import Space

        d = dict(src_dict)
        space = Space.from_dict(d.pop("space"))

        level = Level(d.pop("level"))

        capabilities = []
        _capabilities = d.pop("capabilities")
        for capabilities_item_data in _capabilities:
            capabilities_item = Capability(capabilities_item_data)

            capabilities.append(capabilities_item)

        space_view = cls(
            space=space,
            level=level,
            capabilities=capabilities,
        )

        space_view.additional_properties = d
        return space_view

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
