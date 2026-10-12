from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.host_profile_kind import HostProfileKind

T = TypeVar("T", bound="HostProfile")


@_attrs_define
class HostProfile:
    """Profile reported by the host service (`host_profiles.v1`). Never a command line, path or environment.

    Attributes:
        name (str):
        display_name (str):
        kind (HostProfileKind):
    """

    name: str
    display_name: str
    kind: HostProfileKind

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        display_name = self.display_name

        kind = self.kind.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "display_name": display_name,
                "kind": kind,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        display_name = d.pop("display_name")

        kind = HostProfileKind(d.pop("kind"))

        host_profile = cls(
            name=name,
            display_name=display_name,
            kind=kind,
        )

        return host_profile
