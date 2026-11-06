from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.agent_definition_origin_kind import AgentDefinitionOriginKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentDefinitionOrigin")


@_attrs_define
class AgentDefinitionOrigin:
    """Where the definition came from; display only.

    Attributes:
        kind (AgentDefinitionOriginKind):
        template_slug (str | Unset): `^[a-z0-9][a-z0-9-]{0,63}$`.
        template_version (str | Unset): `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`.
    """

    kind: AgentDefinitionOriginKind
    template_slug: str | Unset = UNSET
    template_version: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        template_slug = self.template_slug

        template_version = self.template_version

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
            }
        )
        if template_slug is not UNSET:
            field_dict["template_slug"] = template_slug
        if template_version is not UNSET:
            field_dict["template_version"] = template_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = AgentDefinitionOriginKind(d.pop("kind"))

        template_slug = d.pop("template_slug", UNSET)

        template_version = d.pop("template_version", UNSET)

        agent_definition_origin = cls(
            kind=kind,
            template_slug=template_slug,
            template_version=template_version,
        )

        return agent_definition_origin
