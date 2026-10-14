from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.template_scope import TemplateScope
from ..types import UNSET, Unset

T = TypeVar("T", bound="Template")


@_attrs_define
class Template:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        scope (TemplateScope):
        name (str):
        content_md (str):
        version (int):
        channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    id: UUID
    scope: TemplateScope
    name: str
    content_md: str
    version: int
    channel_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        scope = self.scope.value

        name = self.name

        content_md = self.content_md

        version = self.version

        channel_id: str | Unset = UNSET
        if not isinstance(self.channel_id, Unset):
            channel_id = str(self.channel_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "scope": scope,
                "name": name,
                "content_md": content_md,
                "version": version,
            }
        )
        if channel_id is not UNSET:
            field_dict["channel_id"] = channel_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        scope = TemplateScope(d.pop("scope"))

        name = d.pop("name")

        content_md = d.pop("content_md")

        version = d.pop("version")

        _channel_id = d.pop("channel_id", UNSET)
        channel_id: UUID | Unset
        if isinstance(_channel_id, Unset):
            channel_id = UNSET
        else:
            channel_id = UUID(_channel_id)

        template = cls(
            id=id,
            scope=scope,
            name=name,
            content_md=content_md,
            version=version,
            channel_id=channel_id,
        )

        template.additional_properties = d
        return template

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
