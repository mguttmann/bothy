from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_definition_version import AgentDefinitionVersion
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    agent_id: UUID,
    *,
    version: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["version"] = version

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/{agent_id}/definition".format(
            agent_id=quote(str(agent_id), safe=""),
        ),
        "params": params,
    }

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
    version: int | Unset = UNSET,
) -> Response[AgentDefinitionVersion | Problem]:
    """Read the current definition of a built-in agent

     The current (highest) version of the definition of the built-in agent `agent_id` (26 4.1), or with
    `version` an older one, which the version list offers for undo (26 8.4). The owner reads it, and the
    agent itself through the hosted connection of its hosting runner, which reads only the current
    version; another agent and a person who is no owner get the refusal of 07 2.13. A built-in agent
    without a definition, any other agent and an unknown `version` answer `404`. The definition appears
    only in this answer, in `putAgentDefinition` and in `current` of its `412`, never in an event, a log
    line, telemetry or an audit entry. `ETag` is the version of the answer; the `If-Match` of the next
    `putAgentDefinition` is the current version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        version (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersion | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        version=version,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    version: int | Unset = UNSET,
) -> AgentDefinitionVersion | Problem | None:
    """Read the current definition of a built-in agent

     The current (highest) version of the definition of the built-in agent `agent_id` (26 4.1), or with
    `version` an older one, which the version list offers for undo (26 8.4). The owner reads it, and the
    agent itself through the hosted connection of its hosting runner, which reads only the current
    version; another agent and a person who is no owner get the refusal of 07 2.13. A built-in agent
    without a definition, any other agent and an unknown `version` answer `404`. The definition appears
    only in this answer, in `putAgentDefinition` and in `current` of its `412`, never in an event, a log
    line, telemetry or an audit entry. `ETag` is the version of the answer; the `If-Match` of the next
    `putAgentDefinition` is the current version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        version (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentDefinitionVersion | Problem
    """

    return sync_detailed(
        agent_id=agent_id,
        client=client,
        version=version,
    ).parsed


async def asyncio_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    version: int | Unset = UNSET,
) -> Response[AgentDefinitionVersion | Problem]:
    """Read the current definition of a built-in agent

     The current (highest) version of the definition of the built-in agent `agent_id` (26 4.1), or with
    `version` an older one, which the version list offers for undo (26 8.4). The owner reads it, and the
    agent itself through the hosted connection of its hosting runner, which reads only the current
    version; another agent and a person who is no owner get the refusal of 07 2.13. A built-in agent
    without a definition, any other agent and an unknown `version` answer `404`. The definition appears
    only in this answer, in `putAgentDefinition` and in `current` of its `412`, never in an event, a log
    line, telemetry or an audit entry. `ETag` is the version of the answer; the `If-Match` of the next
    `putAgentDefinition` is the current version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        version (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentDefinitionVersion | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        version=version,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    version: int | Unset = UNSET,
) -> AgentDefinitionVersion | Problem | None:
    """Read the current definition of a built-in agent

     The current (highest) version of the definition of the built-in agent `agent_id` (26 4.1), or with
    `version` an older one, which the version list offers for undo (26 8.4). The owner reads it, and the
    agent itself through the hosted connection of its hosting runner, which reads only the current
    version; another agent and a person who is no owner get the refusal of 07 2.13. A built-in agent
    without a definition, any other agent and an unknown `version` answer `404`. The definition appears
    only in this answer, in `putAgentDefinition` and in `current` of its `412`, never in an event, a log
    line, telemetry or an audit entry. `ETag` is the version of the answer; the `If-Match` of the next
    `putAgentDefinition` is the current version.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        version (int | Unset):

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
            version=version,
        )
    ).parsed
