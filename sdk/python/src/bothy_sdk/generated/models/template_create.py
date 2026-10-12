from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="TemplateCreate")


@_attrs_define
class TemplateCreate:
    """
    Attributes:
        name (str):
        content_md (str):
    """

    name: str
    content_md: str

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        content_md = self.content_md

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "content_md": content_md,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        content_md = d.pop("content_md")

        template_create = cls(
            name=name,
            content_md=content_md,
        )

        return template_create
