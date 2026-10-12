from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_summary_page import RunSummaryPage
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_agent_id: str | Unset = UNSET
    if not isinstance(agent_id, Unset):
        json_agent_id = str(agent_id)
    params["agent_id"] = json_agent_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/runs",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunSummaryPage:
    if response.status_code == 200:
        response_200 = RunSummaryPage.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | RunSummaryPage]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Response[Problem | RunSummaryPage]:
    """Running now

     Runs in `queued`, `running` or `waiting_input`, filtered by `run.read`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunSummaryPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        agent_id=agent_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Problem | RunSummaryPage | None:
    """Running now

     Runs in `queued`, `running` or `waiting_input`, filtered by `run.read`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunSummaryPage
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        agent_id=agent_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Response[Problem | RunSummaryPage]:
    """Running now

     Runs in `queued`, `running` or `waiting_input`, filtered by `run.read`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunSummaryPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        agent_id=agent_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Problem | RunSummaryPage | None:
    """Running now

     Runs in `queued`, `running` or `waiting_input`, filtered by `run.read`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunSummaryPage
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            agent_id=agent_id,
        )
    ).parsed
