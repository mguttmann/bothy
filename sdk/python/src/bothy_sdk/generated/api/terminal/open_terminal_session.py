from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.terminal_session_grant import TerminalSessionGrant
from ...models.terminal_session_open import TerminalSessionOpen
from ...types import Response


def _get_kwargs(
    host_id: UUID,
    *,
    body: TerminalSessionOpen,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hosts/{host_id}/terminal/sessions".format(
            host_id=quote(str(host_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TerminalSessionGrant:
    if response.status_code == 201:
        response_201 = TerminalSessionGrant.from_dict(response.json())

        return response_201

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
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionOpen,
) -> Response[Problem | TerminalSessionGrant]:
    """Open a terminal session

     Needs a fresh, unused step-up (passkey or step-up device key; in the console only a passkey with
    `userVerification = required`), the `terminal` right, SR10, SR11 (I6) and SR12 (an own session user
    on the host whose name the host reports, otherwise 403 `terminal_denied` with `reason:
    no_session_user`). Consumes the step-up and returns a one-time session token for the Noise handshake
    (24, 5 and 6.1). The device then connects to `stream_path`.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionOpen):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionGrant]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionOpen,
) -> Problem | TerminalSessionGrant | None:
    """Open a terminal session

     Needs a fresh, unused step-up (passkey or step-up device key; in the console only a passkey with
    `userVerification = required`), the `terminal` right, SR10, SR11 (I6) and SR12 (an own session user
    on the host whose name the host reports, otherwise 403 `terminal_denied` with `reason:
    no_session_user`). Consumes the step-up and returns a one-time session token for the Noise handshake
    (24, 5 and 6.1). The device then connects to `stream_path`.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionOpen):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionGrant
    """

    return sync_detailed(
        host_id=host_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionOpen,
) -> Response[Problem | TerminalSessionGrant]:
    """Open a terminal session

     Needs a fresh, unused step-up (passkey or step-up device key; in the console only a passkey with
    `userVerification = required`), the `terminal` right, SR10, SR11 (I6) and SR12 (an own session user
    on the host whose name the host reports, otherwise 403 `terminal_denied` with `reason:
    no_session_user`). Consumes the step-up and returns a one-time session token for the Noise handshake
    (24, 5 and 6.1). The device then connects to `stream_path`.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionOpen):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TerminalSessionGrant]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TerminalSessionOpen,
) -> Problem | TerminalSessionGrant | None:
    """Open a terminal session

     Needs a fresh, unused step-up (passkey or step-up device key; in the console only a passkey with
    `userVerification = required`), the `terminal` right, SR10, SR11 (I6) and SR12 (an own session user
    on the host whose name the host reports, otherwise 403 `terminal_denied` with `reason:
    no_session_user`). Consumes the step-up and returns a one-time session token for the Noise handshake
    (24, 5 and 6.1). The device then connects to `stream_path`.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (TerminalSessionOpen):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TerminalSessionGrant
    """

    return (
        await asyncio_detailed(
            host_id=host_id,
            client=client,
            body=body,
        )
    ).parsed
