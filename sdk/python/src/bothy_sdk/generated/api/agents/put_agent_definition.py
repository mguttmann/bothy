from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_definition import AgentDefinition
from ...models.agent_definition_version import AgentDefinitionVersion
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    agent_id: UUID,
    *,
    body: AgentDefinition,
    if_match: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(if_match, Unset):
        headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/agents/{agent_id}/definition".format(
            agent_id=quote(str(agent_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentDefinitionVersion | Problem:
    if response.status_code == 200:
        response_200 = AgentDefinitionVersion.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 412:
        response_412 = Problem.from_dict(response.json())

        return response_412

    if response.status_code == 428:
        response_428 = Problem.from_dict(response.json())

        return response_428

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AgentDefinitionVersion | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AgentDefinition,
    if_match: str | Unset = UNSET,
) -> Response[AgentDefinitionVersion | Problem]:
    """Store a new version of the definition of a built-in agent

     Part of `agent.update` (owner, step-up, audit; 26 4.1, R-API-29). Stores the body as the next
    version of the definition, raises `agents.version` (`agent.updated`) and sends the durable personal
    event `agent.definition_changed` with `definition_version` only to the agent, all in one
    transaction. `If-Match` names the current definition version and is absent only for the first
    version: without it on an existing definition `428 precondition_required`, a stale one `412
    version_conflict` with the current definition in `current`. Only a built-in agent (`adapter_kind =
    'claude_agent'`) has a definition; another agent answers `409 conflict` with `reason:
    not_a_builtin_agent`. The server checks the body against 26 4.2 and 4.3 and answers every finding as
    one `errors[]` entry of `400 validation_failed` with the JSON Pointer and the field code
    (`unknown_member`, `skill_frontmatter`, `inline_shell`, `import_outside_folder`,
    `secret_value_not_allowed`, `env_name_reserved`, `mcp_command_not_allowed`, `mcp_package_unpinned`,
    `tool_class_too_low`, `too_long`, `too_many`, `invalid_name`, `duplicate_name`, and the shape codes
    of docs/decisions.d/A1-02.md); then it stores nothing. A body equal to the current definition stores
    no new version. The audit entry names the version and the changed parts, never their text. Undo
    writes the content of an older version as a new version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str | Unset):
        body (AgentDefinition): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed
            at every level. Every string is valid UTF-8 without control or bidi characters, except
            line feed and carriage return in `instructions`, a skill `body` and a file `content`. The
            limits are configuration values: `instructions` at most
            `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills`
            skills and `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3
            (frontmatter, inline shell, `@` imports outside the agent folder) and the rules of
            docs/decisions.d/A1-02.md apply; a finding is a field error of `400 validation_failed`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersion | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AgentDefinition,
    if_match: str | Unset = UNSET,
) -> AgentDefinitionVersion | Problem | None:
    """Store a new version of the definition of a built-in agent

     Part of `agent.update` (owner, step-up, audit; 26 4.1, R-API-29). Stores the body as the next
    version of the definition, raises `agents.version` (`agent.updated`) and sends the durable personal
    event `agent.definition_changed` with `definition_version` only to the agent, all in one
    transaction. `If-Match` names the current definition version and is absent only for the first
    version: without it on an existing definition `428 precondition_required`, a stale one `412
    version_conflict` with the current definition in `current`. Only a built-in agent (`adapter_kind =
    'claude_agent'`) has a definition; another agent answers `409 conflict` with `reason:
    not_a_builtin_agent`. The server checks the body against 26 4.2 and 4.3 and answers every finding as
    one `errors[]` entry of `400 validation_failed` with the JSON Pointer and the field code
    (`unknown_member`, `skill_frontmatter`, `inline_shell`, `import_outside_folder`,
    `secret_value_not_allowed`, `env_name_reserved`, `mcp_command_not_allowed`, `mcp_package_unpinned`,
    `tool_class_too_low`, `too_long`, `too_many`, `invalid_name`, `duplicate_name`, and the shape codes
    of docs/decisions.d/A1-02.md); then it stores nothing. A body equal to the current definition stores
    no new version. The audit entry names the version and the changed parts, never their text. Undo
    writes the content of an older version as a new version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str | Unset):
        body (AgentDefinition): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed
            at every level. Every string is valid UTF-8 without control or bidi characters, except
            line feed and carriage return in `instructions`, a skill `body` and a file `content`. The
            limits are configuration values: `instructions` at most
            `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills`
            skills and `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3
            (frontmatter, inline shell, `@` imports outside the agent folder) and the rules of
            docs/decisions.d/A1-02.md apply; a finding is a field error of `400 validation_failed`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentDefinitionVersion | Problem
    """

    return sync_detailed(
        agent_id=agent_id,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AgentDefinition,
    if_match: str | Unset = UNSET,
) -> Response[AgentDefinitionVersion | Problem]:
    """Store a new version of the definition of a built-in agent

     Part of `agent.update` (owner, step-up, audit; 26 4.1, R-API-29). Stores the body as the next
    version of the definition, raises `agents.version` (`agent.updated`) and sends the durable personal
    event `agent.definition_changed` with `definition_version` only to the agent, all in one
    transaction. `If-Match` names the current definition version and is absent only for the first
    version: without it on an existing definition `428 precondition_required`, a stale one `412
    version_conflict` with the current definition in `current`. Only a built-in agent (`adapter_kind =
    'claude_agent'`) has a definition; another agent answers `409 conflict` with `reason:
    not_a_builtin_agent`. The server checks the body against 26 4.2 and 4.3 and answers every finding as
    one `errors[]` entry of `400 validation_failed` with the JSON Pointer and the field code
    (`unknown_member`, `skill_frontmatter`, `inline_shell`, `import_outside_folder`,
    `secret_value_not_allowed`, `env_name_reserved`, `mcp_command_not_allowed`, `mcp_package_unpinned`,
    `tool_class_too_low`, `too_long`, `too_many`, `invalid_name`, `duplicate_name`, and the shape codes
    of docs/decisions.d/A1-02.md); then it stores nothing. A body equal to the current definition stores
    no new version. The audit entry names the version and the changed parts, never their text. Undo
    writes the content of an older version as a new version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str | Unset):
        body (AgentDefinition): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed
            at every level. Every string is valid UTF-8 without control or bidi characters, except
            line feed and carriage return in `instructions`, a skill `body` and a file `content`. The
            limits are configuration values: `instructions` at most
            `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills`
            skills and `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3
            (frontmatter, inline shell, `@` imports outside the agent folder) and the rules of
            docs/decisions.d/A1-02.md apply; a finding is a field error of `400 validation_failed`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersion | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AgentDefinition,
    if_match: str | Unset = UNSET,
) -> AgentDefinitionVersion | Problem | None:
    """Store a new version of the definition of a built-in agent

     Part of `agent.update` (owner, step-up, audit; 26 4.1, R-API-29). Stores the body as the next
    version of the definition, raises `agents.version` (`agent.updated`) and sends the durable personal
    event `agent.definition_changed` with `definition_version` only to the agent, all in one
    transaction. `If-Match` names the current definition version and is absent only for the first
    version: without it on an existing definition `428 precondition_required`, a stale one `412
    version_conflict` with the current definition in `current`. Only a built-in agent (`adapter_kind =
    'claude_agent'`) has a definition; another agent answers `409 conflict` with `reason:
    not_a_builtin_agent`. The server checks the body against 26 4.2 and 4.3 and answers every finding as
    one `errors[]` entry of `400 validation_failed` with the JSON Pointer and the field code
    (`unknown_member`, `skill_frontmatter`, `inline_shell`, `import_outside_folder`,
    `secret_value_not_allowed`, `env_name_reserved`, `mcp_command_not_allowed`, `mcp_package_unpinned`,
    `tool_class_too_low`, `too_long`, `too_many`, `invalid_name`, `duplicate_name`, and the shape codes
    of docs/decisions.d/A1-02.md); then it stores nothing. A body equal to the current definition stores
    no new version. The audit entry names the version and the changed parts, never their text. Undo
    writes the content of an older version as a new version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str | Unset):
        body (AgentDefinition): Schema `agent_definition.v1` of a built-in agent (26 4.2), closed
            at every level. Every string is valid UTF-8 without control or bidi characters, except
            line feed and carriage return in `instructions`, a skill `body` and a file `content`. The
            limits are configuration values: `instructions` at most
            `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills`
            skills and `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3
            (frontmatter, inline shell, `@` imports outside the agent folder) and the rules of
            docs/decisions.d/A1-02.md apply; a finding is a field error of `400 validation_failed`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentDefinitionVersion | Problem
    """

    return (
        await asyncio_detailed(
            agent_id=agent_id,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
