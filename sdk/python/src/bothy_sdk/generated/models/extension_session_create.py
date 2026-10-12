from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.extension_session_create_client_component import (
    ExtensionSessionCreateClientComponent,
)

T = TypeVar("T", bound="ExtensionSessionCreate")


@_attrs_define
class ExtensionSessionCreate:
    """Child session of the calling app session for an app extension with its own refresh family (08, 7.4).

    Attributes:
        client_component (ExtensionSessionCreateClientComponent):
    """

    client_component: ExtensionSessionCreateClientComponent

    def to_dict(self) -> dict[str, Any]:
        client_component = self.client_component.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "client_component": client_component,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        client_component = ExtensionSessionCreateClientComponent(
            d.pop("client_component")
        )

        extension_session_create = cls(
            client_component=client_component,
        )

        return extension_session_create
