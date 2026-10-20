from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.app_scope import AppScope

T = TypeVar("T", bound="AppManifestScopes")


@_attrs_define
class AppManifestScopes:
    """
    Attributes:
        required (list[AppScope]):
        optional (list[AppScope]):
    """

    required: list[AppScope]
    optional: list[AppScope]

    def to_dict(self) -> dict[str, Any]:
        required = []
        for required_item_data in self.required:
            required_item = required_item_data.value
            required.append(required_item)

        optional = []
        for optional_item_data in self.optional:
            optional_item = optional_item_data.value
            optional.append(optional_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "required": required,
                "optional": optional,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        required = []
        _required = d.pop("required")
        for required_item_data in _required:
            required_item = AppScope(required_item_data)

            required.append(required_item)

        optional = []
        _optional = d.pop("optional")
        for optional_item_data in _optional:
            optional_item = AppScope(optional_item_data)

            optional.append(optional_item)

        app_manifest_scopes = cls(
            required=required,
            optional=optional,
        )

        return app_manifest_scopes
