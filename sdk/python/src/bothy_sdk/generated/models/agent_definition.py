from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer
    from ..models.agent_definition_origin import AgentDefinitionOrigin
    from ..models.agent_definition_settings import AgentDefinitionSettings
    from ..models.agent_definition_skill import AgentDefinitionSkill


T = TypeVar("T", bound="AgentDefinition")


@_attrs_define
class AgentDefinition:
    """Schema `agent_definition.v1` of a built-in agent (26 4.2), closed at every level. Every string is valid UTF-8
    without control or bidi characters, except line feed and carriage return in `instructions`, a skill `body` and a
    file `content`. The limits are configuration values: `instructions` at most
    `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills` skills and
    `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3 (frontmatter, inline shell, `@` imports
    outside the agent folder) and the rules of docs/decisions.d/A1-02.md apply; a finding is a field error of `400
    validation_failed`.

        Attributes:
            instructions (str): Markdown that becomes `CLAUDE.md` in the agent folder; at most
                `agents.definition.instructions_max_bytes` bytes.
            skills (list[AgentDefinitionSkill]): At most `agents.definition.max_skills` skills, names unique.
            mcp_servers (list[AgentDefinitionMcpServer]): At most `agents.definition.max_mcp_servers` servers, names unique.
                A `stdio` server needs the tool class `host` (`tool_class_too_low` below it, 26 4.4).
            settings (AgentDefinitionSettings): Settings of the built-in agent; the tool class itself is the agent's
                `tool_class` (07 2.11), never set here.
            origin (AgentDefinitionOrigin): Where the definition came from; display only.
    """

    instructions: str
    skills: list[AgentDefinitionSkill]
    mcp_servers: list[AgentDefinitionMcpServer]
    settings: AgentDefinitionSettings
    origin: AgentDefinitionOrigin

    def to_dict(self) -> dict[str, Any]:
        instructions = self.instructions

        skills = []
        for skills_item_data in self.skills:
            skills_item = skills_item_data.to_dict()
            skills.append(skills_item)

        mcp_servers = []
        for mcp_servers_item_data in self.mcp_servers:
            mcp_servers_item = mcp_servers_item_data.to_dict()
            mcp_servers.append(mcp_servers_item)

        settings = self.settings.to_dict()

        origin = self.origin.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "instructions": instructions,
                "skills": skills,
                "mcp_servers": mcp_servers,
                "settings": settings,
                "origin": origin,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer
        from ..models.agent_definition_origin import AgentDefinitionOrigin
        from ..models.agent_definition_settings import AgentDefinitionSettings
        from ..models.agent_definition_skill import AgentDefinitionSkill

        d = dict(src_dict)
        instructions = d.pop("instructions")

        skills = []
        _skills = d.pop("skills")
        for skills_item_data in _skills:
            skills_item = AgentDefinitionSkill.from_dict(skills_item_data)

            skills.append(skills_item)

        mcp_servers = []
        _mcp_servers = d.pop("mcp_servers")
        for mcp_servers_item_data in _mcp_servers:
            mcp_servers_item = AgentDefinitionMcpServer.from_dict(mcp_servers_item_data)

            mcp_servers.append(mcp_servers_item)

        settings = AgentDefinitionSettings.from_dict(d.pop("settings"))

        origin = AgentDefinitionOrigin.from_dict(d.pop("origin"))

        agent_definition = cls(
            instructions=instructions,
            skills=skills,
            mcp_servers=mcp_servers,
            settings=settings,
            origin=origin,
        )

        return agent_definition
