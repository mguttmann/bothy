from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_interview_catalogue_entry_tool_class_wish import (
    AgentInterviewCatalogueEntryToolClassWish,
)

if TYPE_CHECKING:
    from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer


T = TypeVar("T", bound="AgentInterviewCatalogueEntry")


@_attrs_define
class AgentInterviewCatalogueEntry:
    """
    Attributes:
        slug (str):
        name (str):
        description (str):
        tool_class_wish (AgentInterviewCatalogueEntryToolClassWish):
        mcp_servers (list[AgentDefinitionMcpServer]):
    """

    slug: str
    name: str
    description: str
    tool_class_wish: AgentInterviewCatalogueEntryToolClassWish
    mcp_servers: list[AgentDefinitionMcpServer]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        slug = self.slug

        name = self.name

        description = self.description

        tool_class_wish = self.tool_class_wish.value

        mcp_servers = []
        for mcp_servers_item_data in self.mcp_servers:
            mcp_servers_item = mcp_servers_item_data.to_dict()
            mcp_servers.append(mcp_servers_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "slug": slug,
                "name": name,
                "description": description,
                "tool_class_wish": tool_class_wish,
                "mcp_servers": mcp_servers,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition_mcp_server import AgentDefinitionMcpServer

        d = dict(src_dict)
        slug = d.pop("slug")

        name = d.pop("name")

        description = d.pop("description")

        tool_class_wish = AgentInterviewCatalogueEntryToolClassWish(
            d.pop("tool_class_wish")
        )

        mcp_servers = []
        _mcp_servers = d.pop("mcp_servers")
        for mcp_servers_item_data in _mcp_servers:
            mcp_servers_item = AgentDefinitionMcpServer.from_dict(mcp_servers_item_data)

            mcp_servers.append(mcp_servers_item)

        agent_interview_catalogue_entry = cls(
            slug=slug,
            name=name,
            description=description,
            tool_class_wish=tool_class_wish,
            mcp_servers=mcp_servers,
        )

        agent_interview_catalogue_entry.additional_properties = d
        return agent_interview_catalogue_entry

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
