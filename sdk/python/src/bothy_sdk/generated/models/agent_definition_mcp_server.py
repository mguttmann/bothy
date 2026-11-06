from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.agent_definition_mcp_server_transport import (
    AgentDefinitionMcpServerTransport,
)
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_definition_entry import AgentDefinitionEntry


T = TypeVar("T", bound="AgentDefinitionMcpServer")


@_attrs_define
class AgentDefinitionMcpServer:
    """One MCP server. `http` takes `url` and optional `headers`; `stdio` takes `command`, `package`, optional `args` and
    `env`. A member of the other transport is `unknown_member`. The runner places `package` directly after `command` and
    every entry of `args` after it.

        Attributes:
            name (str): `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the servers.
            transport (AgentDefinitionMcpServerTransport):
            url (str | Unset): `http`: an `https` URL with a host, without user information and without fragment.
            headers (list[AgentDefinitionEntry] | Unset): `http`: headers, names unique without case.
            command (str | Unset): `stdio`: one of `agents.mcp_stdio_runners` (`mcp_command_not_allowed`).
            package (str | Unset): `stdio`: `<name>@<exact version>`, a semantic version without range, wildcard or tag
                (`mcp_package_unpinned`).
            args (list[str] | Unset): `stdio`: arguments after the package.
            env (list[AgentDefinitionEntry] | Unset): `stdio`: environment entries, names `^[A-Z_][A-Z0-9_]*$`, unique,
                never a name the runner sets itself (`env_name_reserved`).
    """

    name: str
    transport: AgentDefinitionMcpServerTransport
    url: str | Unset = UNSET
    headers: list[AgentDefinitionEntry] | Unset = UNSET
    command: str | Unset = UNSET
    package: str | Unset = UNSET
    args: list[str] | Unset = UNSET
    env: list[AgentDefinitionEntry] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        transport = self.transport.value

        url = self.url

        headers: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.headers, Unset):
            headers = []
            for headers_item_data in self.headers:
                headers_item = headers_item_data.to_dict()
                headers.append(headers_item)

        command = self.command

        package = self.package

        args: list[str] | Unset = UNSET
        if not isinstance(self.args, Unset):
            args = self.args

        env: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.env, Unset):
            env = []
            for env_item_data in self.env:
                env_item = env_item_data.to_dict()
                env.append(env_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "transport": transport,
            }
        )
        if url is not UNSET:
            field_dict["url"] = url
        if headers is not UNSET:
            field_dict["headers"] = headers
        if command is not UNSET:
            field_dict["command"] = command
        if package is not UNSET:
            field_dict["package"] = package
        if args is not UNSET:
            field_dict["args"] = args
        if env is not UNSET:
            field_dict["env"] = env

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_definition_entry import AgentDefinitionEntry

        d = dict(src_dict)
        name = d.pop("name")

        transport = AgentDefinitionMcpServerTransport(d.pop("transport"))

        url = d.pop("url", UNSET)

        _headers = d.pop("headers", UNSET)
        headers: list[AgentDefinitionEntry] | Unset = UNSET
        if _headers is not UNSET:
            headers = []
            for headers_item_data in _headers:
                headers_item = AgentDefinitionEntry.from_dict(headers_item_data)

                headers.append(headers_item)

        command = d.pop("command", UNSET)

        package = d.pop("package", UNSET)

        args = cast(list[str], d.pop("args", UNSET))

        _env = d.pop("env", UNSET)
        env: list[AgentDefinitionEntry] | Unset = UNSET
        if _env is not UNSET:
            env = []
            for env_item_data in _env:
                env_item = AgentDefinitionEntry.from_dict(env_item_data)

                env.append(env_item)

        agent_definition_mcp_server = cls(
            name=name,
            transport=transport,
            url=url,
            headers=headers,
            command=command,
            package=package,
            args=args,
            env=env,
        )

        return agent_definition_mcp_server
