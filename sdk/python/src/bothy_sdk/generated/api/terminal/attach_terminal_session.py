from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.terminal_session_attach import TerminalSessionAttach
from ...models.terminal_session_grant import TerminalSessionGrant
from ...types import Response


def _get_kwargs(
    session_id: UUID,
    *,
    body: TerminalSessionAttach,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/terminal/sessions/{session_id}/attach".format(
            session_id=quote(str(session_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TerminalSessionGrant:
    if response.status_code == 200:
        response_200 = TerminalSessionGrant.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | TerminalSessionGrant]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionAttach,
) -> Response[Problem | TerminalSessionGrant]:
    """Attach to an own session again

     For a session in state `opening`, `active` or `detached`. Needs a fresh step-up, except from the
    same device in the same login session within `terminal.quick_reattach_window` after the disconnect.
    Returns a new session token; older tokens become worthless. A connection of another device is closed
    with `4103` (24, 6.2). An attach to an `opening` session restarts the opening (`att = open`, new
    deadline).

    Args:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionAttach):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionGrant]
    """

    kwargs = _get_kwargs(
        session_id=session_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionAttach,
) -> Problem | TerminalSessionGrant | None:
    """Attach to an own session again

     For a session in state `opening`, `active` or `detached`. Needs a fresh step-up, except from the
    same device in the same login session within `terminal.quick_reattach_window` after the disconnect.
    Returns a new session token; older tokens become worthless. A connection of another device is closed
    with `4103` (24, 6.2). An attach to an `opening` session restarts the opening (`att = open`, new
    deadline).

    Args:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionAttach):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionGrant
    """

    return sync_detailed(
        session_id=session_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionAttach,
) -> Response[Problem | TerminalSessionGrant]:
    """Attach to an own session again

     For a session in state `opening`, `active` or `detached`. Needs a fresh step-up, except from the
    same device in the same login session within `terminal.quick_reattach_window` after the disconnect.
    Returns a new session token; older tokens become worthless. A connection of another device is closed
    with `4103` (24, 6.2). An attach to an `opening` session restarts the opening (`att = open`, new
    deadline).

    Args:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionAttach):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionGrant]
    """

    kwargs = _get_kwargs(
        session_id=session_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    session_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionAttach,
) -> Problem | TerminalSessionGrant | None:
    """Attach to an own session again

     For a session in state `opening`, `active` or `detached`. Needs a fresh step-up, except from the
    same device in the same login session within `terminal.quick_reattach_window` after the disconnect.
    Returns a new session token; older tokens become worthless. A connection of another device is closed
    with `4103` (24, 6.2). An attach to an `opening` session restarts the opening (`att = open`, new
    deadline).

    Args:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionAttach):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionGrant
    """

    return (
        await asyncio_detailed(
            session_id=session_id,
            client=client,
            body=body,
        )
    ).parsed
