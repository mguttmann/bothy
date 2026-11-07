from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_template_list import AgentTemplateList
from ...models.problem import Problem
from ...types import Response


def _get_kwargs() -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agent-templates",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentTemplateList | Problem:
    if response.status_code == 200:
        response_200 = AgentTemplateList.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AgentTemplateList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[AgentTemplateList | Problem]:
    """List the agent templates

     The templates for a new built-in agent (26 7): the shipped templates of `templates/agents/`,
    embedded in the server, then the private templates of the folder `agents.templates_dir`, each
    ordered by `slug`. The server reads both once at start; a private template whose slug a shipped one
    already has, or that fails the schema `agent_template.v1` or the rules of 26 4.3, is left out and
    logged with `template_slug_taken` or `template_invalid` (26 7.2). `definition` is the template as a
    definition draft with `origin.kind` `template`, ready for `putAgentDefinition` (26 7.1). A small
    unpaginated list (06, 6 point 6). Owner only.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentTemplateList | Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> AgentTemplateList | Problem | None:
    """List the agent templates

     The templates for a new built-in agent (26 7): the shipped templates of `templates/agents/`,
    embedded in the server, then the private templates of the folder `agents.templates_dir`, each
    ordered by `slug`. The server reads both once at start; a private template whose slug a shipped one
    already has, or that fails the schema `agent_template.v1` or the rules of 26 4.3, is left out and
    logged with `template_slug_taken` or `template_invalid` (26 7.2). `definition` is the template as a
    definition draft with `origin.kind` `template`, ready for `putAgentDefinition` (26 7.1). A small
    unpaginated list (06, 6 point 6). Owner only.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentTemplateList | Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[AgentTemplateList | Problem]:
    """List the agent templates

     The templates for a new built-in agent (26 7): the shipped templates of `templates/agents/`,
    embedded in the server, then the private templates of the folder `agents.templates_dir`, each
    ordered by `slug`. The server reads both once at start; a private template whose slug a shipped one
    already has, or that fails the schema `agent_template.v1` or the rules of 26 4.3, is left out and
    logged with `template_slug_taken` or `template_invalid` (26 7.2). `definition` is the template as a
    definition draft with `origin.kind` `template`, ready for `putAgentDefinition` (26 7.1). A small
    unpaginated list (06, 6 point 6). Owner only.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentTemplateList | Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> AgentTemplateList | Problem | None:
    """List the agent templates

     The templates for a new built-in agent (26 7): the shipped templates of `templates/agents/`,
    embedded in the server, then the private templates of the folder `agents.templates_dir`, each
    ordered by `slug`. The server reads both once at start; a private template whose slug a shipped one
    already has, or that fails the schema `agent_template.v1` or the rules of 26 4.3, is left out and
    logged with `template_slug_taken` or `template_invalid` (26 7.2). `definition` is the template as a
    definition draft with `origin.kind` `template`, ready for `putAgentDefinition` (26 7.1). A small
    unpaginated list (06, 6 point 6). Owner only.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentTemplateList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
