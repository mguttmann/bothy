from http import HTTPStatus
from io import BytesIO
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...types import UNSET, File, Response, Unset


def _get_kwargs(
    file_id: UUID,
    *,
    ticket: str | Unset = UNSET,
    range_: str | Unset = UNSET,
    if_range: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(range_, Unset):
        headers["Range"] = range_

    if not isinstance(if_range, Unset):
        headers["If-Range"] = if_range

    params: dict[str, Any] = {}

    params["ticket"] = ticket

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/files/{file_id}/content".format(
            file_id=quote(str(file_id), safe=""),
        ),
        "params": params,
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> File | Problem:
    if response.status_code == 200:
        response_200 = File(payload=BytesIO(response.content))

        return response_200

    if response.status_code == 206:
        response_206 = File(payload=BytesIO(response.content))

        return response_206

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

    if response.status_code == 416:
        response_416 = Problem.from_dict(response.json())

        return response_416

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[File | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient,
    ticket: str | Unset = UNSET,
    range_: str | Unset = UNSET,
    if_range: str | Unset = UNSET,
) -> Response[File | Problem]:
    """Download a file

     Answers 200 with the bytes (supports `Range`); there are no presigned GET URLs (ADR 0048).
    Media players without DPoP use a ticket (`createFileTicket`) in the query parameter `ticket`
    instead of `Authorization`. 409 `file_not_ready` while the scan is pending, 409 `file_blocked`
    for blocked or failed files; without read access always 404 (12, 9.3). Exactly one byte range
    gives 206, several ranges are ignored (200 with the whole file), a range outside the file
    gives 416. Types outside the inline list of 12 9.3 (HTML, SVG, PDF and every other type) are sent as
    `application/octet-stream` with `Content-Disposition: attachment`, `nosniff` and the sandbox
    CSP. An `Authorization` header wins over a `ticket` parameter. Pass-through via the server
    only (ADR 0048).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ticket (str | Unset):
        range_ (str | Unset):
        if_range (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        ticket=ticket,
        range_=range_,
        if_range=if_range,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    *,
    client: AuthenticatedClient,
    ticket: str | Unset = UNSET,
    range_: str | Unset = UNSET,
    if_range: str | Unset = UNSET,
) -> File | Problem | None:
    """Download a file

     Answers 200 with the bytes (supports `Range`); there are no presigned GET URLs (ADR 0048).
    Media players without DPoP use a ticket (`createFileTicket`) in the query parameter `ticket`
    instead of `Authorization`. 409 `file_not_ready` while the scan is pending, 409 `file_blocked`
    for blocked or failed files; without read access always 404 (12, 9.3). Exactly one byte range
    gives 206, several ranges are ignored (200 with the whole file), a range outside the file
    gives 416. Types outside the inline list of 12 9.3 (HTML, SVG, PDF and every other type) are sent as
    `application/octet-stream` with `Content-Disposition: attachment`, `nosniff` and the sandbox
    CSP. An `Authorization` header wins over a `ticket` parameter. Pass-through via the server
    only (ADR 0048).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ticket (str | Unset):
        range_ (str | Unset):
        if_range (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return sync_detailed(
        file_id=file_id,
        client=client,
        ticket=ticket,
        range_=range_,
        if_range=if_range,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient,
    ticket: str | Unset = UNSET,
    range_: str | Unset = UNSET,
    if_range: str | Unset = UNSET,
) -> Response[File | Problem]:
    """Download a file

     Answers 200 with the bytes (supports `Range`); there are no presigned GET URLs (ADR 0048).
    Media players without DPoP use a ticket (`createFileTicket`) in the query parameter `ticket`
    instead of `Authorization`. 409 `file_not_ready` while the scan is pending, 409 `file_blocked`
    for blocked or failed files; without read access always 404 (12, 9.3). Exactly one byte range
    gives 206, several ranges are ignored (200 with the whole file), a range outside the file
    gives 416. Types outside the inline list of 12 9.3 (HTML, SVG, PDF and every other type) are sent as
    `application/octet-stream` with `Content-Disposition: attachment`, `nosniff` and the sandbox
    CSP. An `Authorization` header wins over a `ticket` parameter. Pass-through via the server
    only (ADR 0048).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ticket (str | Unset):
        range_ (str | Unset):
        if_range (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        ticket=ticket,
        range_=range_,
        if_range=if_range,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    *,
    client: AuthenticatedClient,
    ticket: str | Unset = UNSET,
    range_: str | Unset = UNSET,
    if_range: str | Unset = UNSET,
) -> File | Problem | None:
    """Download a file

     Answers 200 with the bytes (supports `Range`); there are no presigned GET URLs (ADR 0048).
    Media players without DPoP use a ticket (`createFileTicket`) in the query parameter `ticket`
    instead of `Authorization`. 409 `file_not_ready` while the scan is pending, 409 `file_blocked`
    for blocked or failed files; without read access always 404 (12, 9.3). Exactly one byte range
    gives 206, several ranges are ignored (200 with the whole file), a range outside the file
    gives 416. Types outside the inline list of 12 9.3 (HTML, SVG, PDF and every other type) are sent as
    `application/octet-stream` with `Content-Disposition: attachment`, `nosniff` and the sandbox
    CSP. An `Authorization` header wins over a `ticket` parameter. Pass-through via the server
    only (ADR 0048).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ticket (str | Unset):
        range_ (str | Unset):
        if_range (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            client=client,
            ticket=ticket,
            range_=range_,
            if_range=if_range,
        )
    ).parsed
