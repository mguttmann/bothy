from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer
    from ..models.agent_definition_origin import AgentDefinitionOrigin
    from ..models.agent_definition_settings import AgentDefinitionSettings
    from ..models.agent_definition_skill import AgentDefinitionSkill
    from ..models.agent_draft_channel import AgentDraftChannel


T = TypeVar("T", bound="AgentDraft")


@_attrs_define
class AgentDraft:
    """A definition draft of the builder (26 6.1 item 2): the members of `AgentDefinition` plus the agent's name, handle,
    description and an optional channel suggestion. The server checks it like a definition for the class of
    `settings.tool_class_wish` and against the template catalogue; it never stores it.

        Attributes:
            instructions (str): Markdown that becomes `CLAUDE.md`; at most `agents.definition.instructions_max_bytes` bytes.
            skills (list[AgentDefinitionSkill]): At most `agents.definition.max_skills` skills.
            mcp_servers (list[AgentDefinitionMcpServer]): At most `agents.definition.max_mcp_servers` servers, each equal to
                a server of the catalogue except `name`.
            settings (AgentDefinitionSettings): Settings of the built-in agent; the tool class itself is the agent's
                `tool_class` (07 2.11), never set here.
            origin (AgentDefinitionOrigin): Where the definition came from; display only.
            name (str): The display name the builder suggests.
            handle (str): The handle the builder suggests; `createAgent` checks it.
            description (str): One line about what the agent does.
            channel_suggestion (AgentDraftChannel | Unset): Where people talk to the new agent; the creation flow offers to
                create this channel (26 8.2).
    """

    instructions: str
    skills: list[AgentDefinitionSkill]
    mcp_servers: list[AgentDefinitionMcpServer]
    settings: AgentDefinitionSettings
    origin: AgentDefinitionOrigin
    name: str
    handle: str
    description: str
    channel_suggestion: AgentDraftChannel | Unset = UNSET

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

        name = self.name

        handle = self.handle

        description = self.description

        channel_suggestion: dict[str, Any] | Unset = UNSET
        if not isinstance(self.channel_suggestion, Unset):
            channel_suggestion = self.channel_suggestion.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "instructions": instructions,
                "skills": skills,
                "mcp_servers": mcp_servers,
                "settings": settings,
                "origin": origin,
                "name": name,
                "handle": handle,
                "description": description,
            }
        )
        if channel_suggestion is not UNSET:
            field_dict["channel_suggestion"] = channel_suggestion

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer
        from ..models.agent_definition_origin import AgentDefinitionOrigin
        from ..models.agent_definition_settings import AgentDefinitionSettings
        from ..models.agent_definition_skill import AgentDefinitionSkill
        from ..models.agent_draft_channel import AgentDraftChannel

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

        name = d.pop("name")

        handle = d.pop("handle")

        description = d.pop("description")

        _channel_suggestion = d.pop("channel_suggestion", UNSET)
        channel_suggestion: AgentDraftChannel | Unset
        if isinstance(_channel_suggestion, Unset):
            channel_suggestion = UNSET
        else:
            channel_suggestion = AgentDraftChannel.from_dict(_channel_suggestion)

        agent_draft = cls(
            instructions=instructions,
            skills=skills,
            mcp_servers=mcp_servers,
            settings=settings,
            origin=origin,
            name=name,
            handle=handle,
            description=description,
            channel_suggestion=channel_suggestion,
        )

        return agent_draft
