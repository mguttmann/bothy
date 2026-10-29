from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.project_folder_list import ProjectFolderList
from ...types import Response


def _get_kwargs(
    agent_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/{agent_id}/project-folders".format(
            agent_id=quote(str(agent_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ProjectFolderList:
    if response.status_code == 200:
        response_200 = ProjectFolderList.from_dict(response.json())

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
) -> Response[Problem | ProjectFolderList]:
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
) -> Response[Problem | ProjectFolderList]:
    """List the project folders of a runner that have no binding

     The folders directly under the projects root of the runner `agent_id` that its last heartbeat
    reported (`Heartbeat.project_folders`, 09 18.13) and that no forum is bound to: a name equal to the
    `workspace_ref` of any binding, compared without case, is left out (03 `runner_project_folders`,
    M4-44). `reported` is false while the runner has never reported its folders. The app offers these
    folders to add a forum for each (01 6.10); the server never reads a host's files.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ProjectFolderList]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ProjectFolderList | None:
    """List the project folders of a runner that have no binding

     The folders directly under the projects root of the runner `agent_id` that its last heartbeat
    reported (`Heartbeat.project_folders`, 09 18.13) and that no forum is bound to: a name equal to the
    `workspace_ref` of any binding, compared without case, is left out (03 `runner_project_folders`,
    M4-44). `reported` is false while the runner has never reported its folders. The app offers these
    folders to add a forum for each (01 6.10); the server never reads a host's files.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ProjectFolderList
    """

    return sync_detailed(
        agent_id=agent_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | ProjectFolderList]:
    """List the project folders of a runner that have no binding

     The folders directly under the projects root of the runner `agent_id` that its last heartbeat
    reported (`Heartbeat.project_folders`, 09 18.13) and that no forum is bound to: a name equal to the
    `workspace_ref` of any binding, compared without case, is left out (03 `runner_project_folders`,
    M4-44). `reported` is false while the runner has never reported its folders. The app offers these
    folders to add a forum for each (01 6.10); the server never reads a host's files.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ProjectFolderList]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ProjectFolderList | None:
    """List the project folders of a runner that have no binding

     The folders directly under the projects root of the runner `agent_id` that its last heartbeat
    reported (`Heartbeat.project_folders`, 09 18.13) and that no forum is bound to: a name equal to the
    `workspace_ref` of any binding, compared without case, is left out (03 `runner_project_folders`,
    M4-44). `reported` is false while the runner has never reported its folders. The app offers these
    folders to add a forum for each (01 6.10); the server never reads a host's files.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ProjectFolderList
    """

    return (
        await asyncio_detailed(
            agent_id=agent_id,
            client=client,
        )
    ).parsed
