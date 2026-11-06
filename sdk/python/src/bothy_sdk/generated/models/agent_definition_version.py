from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_definition_version_origin_kind import (
    AgentDefinitionVersionOriginKind,
)
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_definition import AgentDefinition


T = TypeVar("T", bound="AgentDefinitionVersion")


@_attrs_define
class AgentDefinitionVersion:
    """One stored version of the definition of a built-in agent; `definition` only in `getAgentDefinition` and
    `putAgentDefinition`.

        Attributes:
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            version (int):
            origin_kind (AgentDefinitionVersionOriginKind):
            created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            created_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            definition (AgentDefinition | Unset): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed at every
                level. Every string is valid UTF-8 without control or bidi characters, except line feed and carriage return in
                `instructions`, a skill `body` and a file `content`. The limits are configuration values: `instructions` at most
                `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills` skills and
                `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3 (frontmatter, inline shell, `@`
                imports outside the agent folder) and the rules of docs/decisions.d/A1-02.md apply; a finding is a field error
                of `400 validation_failed`.
    """

    agent_id: UUID
    version: int
    origin_kind: AgentDefinitionVersionOriginKind
    created_at: datetime.datetime
    created_by: UUID
    definition: AgentDefinition | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        version = self.version

        origin_kind = self.origin_kind.value

        created_at = self.created_at.isoformat()

        created_by = str(self.created_by)

        definition: dict[str, Any] | Unset = UNSET
        if not isinstance(self.definition, Unset):
            definition = self.definition.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_id": agent_id,
                "version": version,
                "origin_kind": origin_kind,
                "created_at": created_at,
                "created_by": created_by,
            }
        )
        if definition is not UNSET:
            field_dict["definition"] = definition

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition import AgentDefinition

        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        version = d.pop("version")

        origin_kind = AgentDefinitionVersionOriginKind(d.pop("origin_kind"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        created_by = UUID(d.pop("created_by"))

        _definition = d.pop("definition", UNSET)
        definition: AgentDefinition | Unset
        if isinstance(_definition, Unset):
            definition = UNSET
        else:
            definition = AgentDefinition.from_dict(_definition)

        agent_definition_version = cls(
            agent_id=agent_id,
            version=version,
            origin_kind=origin_kind,
            created_at=created_at,
            created_by=created_by,
            definition=definition,
        )

        agent_definition_version.additional_properties = d
        return agent_definition_version

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
