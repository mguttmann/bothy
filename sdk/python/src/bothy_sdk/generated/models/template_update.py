from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="TemplateUpdate")


@_attrs_define
class TemplateUpdate:
    """
    Attributes:
        name (str | Unset):
        content_md (str | Unset):
    """

    name: str | Unset = UNSET
    content_md: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        content_md = self.content_md

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if content_md is not UNSET:
            field_dict["content_md"] = content_md

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        content_md = d.pop("content_md", UNSET)

        template_update = cls(
            name=name,
            content_md=content_md,
        )

        return template_update
