from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.file_ticket import FileTicket
from ...models.file_ticket_create import FileTicketCreate
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    file_id: UUID,
    *,
    body: FileTicketCreate,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/files/{file_id}/tickets".format(
            file_id=quote(str(file_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> FileTicket | Problem:
    if response.status_code == 201:
        response_201 = FileTicket.from_dict(response.json())

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
) -> Response[FileTicket | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FileTicketCreate,
) -> Response[FileTicket | Problem]:
    """Create a short-lived fetch ticket

     For media players and "open with" that cannot send a DPoP proof. Every fetch with the ticket
    checks signature, expiry, session and `file.read` again (12, 9.2). A ticket binds to the login
    session of the caller: a principal without a session (agent, app, API token) gets 403
    `forbidden` with `reason` `step_up_not_possible`. No ticket is issued for a file that is not
    deliverable (409 `file_not_ready` or `file_blocked`), and a variant the file does not have
    gives 404. A repeated call mints a new ticket.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (FileTicketCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[FileTicket | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FileTicketCreate,
) -> FileTicket | Problem | None:
    """Create a short-lived fetch ticket

     For media players and "open with" that cannot send a DPoP proof. Every fetch with the ticket
    checks signature, expiry, session and `file.read` again (12, 9.2). A ticket binds to the login
    session of the caller: a principal without a session (agent, app, API token) gets 403
    `forbidden` with `reason` `step_up_not_possible`. No ticket is issued for a file that is not
    deliverable (409 `file_not_ready` or `file_blocked`), and a variant the file does not have
    gives 404. A repeated call mints a new ticket.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (FileTicketCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        FileTicket | Problem
    """

    return sync_detailed(
        file_id=file_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FileTicketCreate,
) -> Response[FileTicket | Problem]:
    """Create a short-lived fetch ticket

     For media players and "open with" that cannot send a DPoP proof. Every fetch with the ticket
    checks signature, expiry, session and `file.read` again (12, 9.2). A ticket binds to the login
    session of the caller: a principal without a session (agent, app, API token) gets 403
    `forbidden` with `reason` `step_up_not_possible`. No ticket is issued for a file that is not
    deliverable (409 `file_not_ready` or `file_blocked`), and a variant the file does not have
    gives 404. A repeated call mints a new ticket.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (FileTicketCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[FileTicket | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: FileTicketCreate,
) -> FileTicket | Problem | None:
    """Create a short-lived fetch ticket

     For media players and "open with" that cannot send a DPoP proof. Every fetch with the ticket
    checks signature, expiry, session and `file.read` again (12, 9.2). A ticket binds to the login
    session of the caller: a principal without a session (agent, app, API token) gets 403
    `forbidden` with `reason` `step_up_not_possible`. No ticket is issued for a file that is not
    deliverable (409 `file_not_ready` or `file_blocked`), and a variant the file does not have
    gives 404. A repeated call mints a new ticket.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (FileTicketCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        FileTicket | Problem
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            client=client,
            body=body,
        )
    ).parsed
