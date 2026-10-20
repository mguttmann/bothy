from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.capability import Capability
from ..models.level import Level
from ..models.target_kind import TargetKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="AppManifestRequestedGrantsItem")


@_attrs_define
class AppManifestRequestedGrantsItem:
    """
    Attributes:
        target_kind (TargetKind): `host` is a connected host for the terminal; it has no level and only the capability
            `terminal` (24, 8.1).
        level (Level): Grant level. Ordered `read` < `write` < `trigger`.
        purpose (str):
        capabilities (list[Capability] | Unset):
    """

    target_kind: TargetKind
    level: Level
    purpose: str
    capabilities: list[Capability] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        target_kind = self.target_kind.value

        level = self.level.value

        purpose = self.purpose

        capabilities: list[str] | Unset = UNSET
        if not isinstance(self.capabilities, Unset):
            capabilities = []
            for capabilities_item_data in self.capabilities:
                capabilities_item = capabilities_item_data.value
                capabilities.append(capabilities_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "target_kind": target_kind,
                "level": level,
                "purpose": purpose,
            }
        )
        if capabilities is not UNSET:
            field_dict["capabilities"] = capabilities

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        target_kind = TargetKind(d.pop("target_kind"))

        level = Level(d.pop("level"))

        purpose = d.pop("purpose")

        _capabilities = d.pop("capabilities", UNSET)
        capabilities: list[Capability] | Unset = UNSET
        if _capabilities is not UNSET:
            capabilities = []
            for capabilities_item_data in _capabilities:
                capabilities_item = Capability(capabilities_item_data)

                capabilities.append(capabilities_item)

        app_manifest_requested_grants_item = cls(
            target_kind=target_kind,
            level=level,
            purpose=purpose,
            capabilities=capabilities,
        )

        return app_manifest_requested_grants_item
