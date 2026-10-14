from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.terminal_session_page import TerminalSessionPage
from ...models.terminal_session_state import TerminalSessionState
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    host_id: UUID | Unset = UNSET,
    state: TerminalSessionState | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_host_id: str | Unset = UNSET
    if not isinstance(host_id, Unset):
        json_host_id = str(host_id)
    params["host_id"] = json_host_id

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/terminal/sessions",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TerminalSessionPage:
    if response.status_code == 200:
        response_200 = TerminalSessionPage.from_dict(response.json())

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
) -> Response[Problem | TerminalSessionPage]:
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
    host_id: UUID | Unset = UNSET,
    state: TerminalSessionState | Unset = UNSET,
) -> Response[Problem | TerminalSessionPage]:
    """List terminal sessions

     Own sessions; owners see all sessions. Metadata only, never content.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        host_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (TerminalSessionState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        host_id=host_id,
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
    host_id: UUID | Unset = UNSET,
    state: TerminalSessionState | Unset = UNSET,
) -> Problem | TerminalSessionPage | None:
    """List terminal sessions

     Own sessions; owners see all sessions. Metadata only, never content.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        host_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (TerminalSessionState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionPage
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        host_id=host_id,
        state=state,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    host_id: UUID | Unset = UNSET,
    state: TerminalSessionState | Unset = UNSET,
) -> Response[Problem | TerminalSessionPage]:
    """List terminal sessions

     Own sessions; owners see all sessions. Metadata only, never content.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        host_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (TerminalSessionState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        host_id=host_id,
        state=state,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    host_id: UUID | Unset = UNSET,
    state: TerminalSessionState | Unset = UNSET,
) -> Problem | TerminalSessionPage | None:
    """List terminal sessions

     Own sessions; owners see all sessions. Metadata only, never content.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        host_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (TerminalSessionState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionPage
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            host_id=host_id,
            state=state,
        )
    ).parsed
