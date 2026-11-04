from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.list_run_history_state import ListRunHistoryState
from ...models.problem import Problem
from ...models.run_history_page import RunHistoryPage
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    state: ListRunHistoryState | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    json_agent_id: str | Unset = UNSET
    if not isinstance(agent_id, Unset):
        json_agent_id = str(agent_id)
    params["agent_id"] = json_agent_id

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/run-history",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunHistoryPage:
    if response.status_code == 200:
        response_200 = RunHistoryPage.from_dict(response.json())

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
) -> Response[Problem | RunHistoryPage]:
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
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    state: ListRunHistoryState | Unset = UNSET,
) -> Response[Problem | RunHistoryPage]:
    """Finished runs, newest first

     Runs in `completed`, `failed` or `cancelled` across every thread the caller may read, ordered by
    `finished_at` descending, then by ID; each run passes `run.read` in filter mode, as in
    `listActiveRuns` (docs/spec/25-visualization.md, section 6.7). A `space_id` or `agent_id` the caller
    cannot see gives an empty page. Runs stay as long as their thread: there is no time-based deletion
    of runs, so the last page reaches the oldest run the caller may read.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (ListRunHistoryState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunHistoryPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        agent_id=agent_id,
        state=state,
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
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    state: ListRunHistoryState | Unset = UNSET,
) -> Problem | RunHistoryPage | None:
    """Finished runs, newest first

     Runs in `completed`, `failed` or `cancelled` across every thread the caller may read, ordered by
    `finished_at` descending, then by ID; each run passes `run.read` in filter mode, as in
    `listActiveRuns` (docs/spec/25-visualization.md, section 6.7). A `space_id` or `agent_id` the caller
    cannot see gives an empty page. Runs stay as long as their thread: there is no time-based deletion
    of runs, so the last page reaches the oldest run the caller may read.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (ListRunHistoryState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunHistoryPage
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        agent_id=agent_id,
        state=state,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    state: ListRunHistoryState | Unset = UNSET,
) -> Response[Problem | RunHistoryPage]:
    """Finished runs, newest first

     Runs in `completed`, `failed` or `cancelled` across every thread the caller may read, ordered by
    `finished_at` descending, then by ID; each run passes `run.read` in filter mode, as in
    `listActiveRuns` (docs/spec/25-visualization.md, section 6.7). A `space_id` or `agent_id` the caller
    cannot see gives an empty page. Runs stay as long as their thread: there is no time-based deletion
    of runs, so the last page reaches the oldest run the caller may read.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (ListRunHistoryState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunHistoryPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        agent_id=agent_id,
        state=state,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    state: ListRunHistoryState | Unset = UNSET,
) -> Problem | RunHistoryPage | None:
    """Finished runs, newest first

     Runs in `completed`, `failed` or `cancelled` across every thread the caller may read, ordered by
    `finished_at` descending, then by ID; each run passes `run.read` in filter mode, as in
    `listActiveRuns` (docs/spec/25-visualization.md, section 6.7). A `space_id` or `agent_id` the caller
    cannot see gives an empty page. Runs stay as long as their thread: there is no time-based deletion
    of runs, so the last page reaches the oldest run the caller may read.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (ListRunHistoryState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunHistoryPage
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            space_id=space_id,
            agent_id=agent_id,
            state=state,
        )
    ).parsed
