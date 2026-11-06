from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_definition_version_page import AgentDefinitionVersionPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    agent_id: UUID,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/{agent_id}/definition/versions".format(
            agent_id=quote(str(agent_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentDefinitionVersionPage | Problem:
    if response.status_code == 200:
        response_200 = AgentDefinitionVersionPage.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AgentDefinitionVersionPage | Problem]:
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
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[AgentDefinitionVersionPage | Problem]:
    """List the versions of the definition of a built-in agent

     The stored versions of the definition of the built-in agent `agent_id`, newest first, without the
    definition text (26 4.1). The version list offers undo: the client reads the content of an older
    version with `getAgentDefinition` and `version`, and writes it as a new version with
    `putAgentDefinition` (26 8.4). Permissions as `getAgentDefinition`; a built-in agent without a
    definition answers an empty list, any other agent `404`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersionPage | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        cursor=cursor,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> AgentDefinitionVersionPage | Problem | None:
    """List the versions of the definition of a built-in agent

     The stored versions of the definition of the built-in agent `agent_id`, newest first, without the
    definition text (26 4.1). The version list offers undo: the client reads the content of an older
    version with `getAgentDefinition` and `version`, and writes it as a new version with
    `putAgentDefinition` (26 8.4). Permissions as `getAgentDefinition`; a built-in agent without a
    definition answers an empty list, any other agent `404`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentDefinitionVersionPage | Problem
    """

    return sync_detailed(
        agent_id=agent_id,
        client=client,
        cursor=cursor,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[AgentDefinitionVersionPage | Problem]:
    """List the versions of the definition of a built-in agent

     The stored versions of the definition of the built-in agent `agent_id`, newest first, without the
    definition text (26 4.1). The version list offers undo: the client reads the content of an older
    version with `getAgentDefinition` and `version`, and writes it as a new version with
    `putAgentDefinition` (26 8.4). Permissions as `getAgentDefinition`; a built-in agent without a
    definition answers an empty list, any other agent `404`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersionPage | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        cursor=cursor,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> AgentDefinitionVersionPage | Problem | None:
    """List the versions of the definition of a built-in agent

     The stored versions of the definition of the built-in agent `agent_id`, newest first, without the
    definition text (26 4.1). The version list offers undo: the client reads the content of an older
    version with `getAgentDefinition` and `version`, and writes it as a new version with
    `putAgentDefinition` (26 8.4). Permissions as `getAgentDefinition`; a built-in agent without a
    definition answers an empty list, any other agent `404`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentDefinitionVersionPage | Problem
    """

    return (
        await asyncio_detailed(
            agent_id=agent_id,
            client=client,
            cursor=cursor,
            limit=limit,
        )
    ).parsed
