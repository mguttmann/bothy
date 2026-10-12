from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.transcript import Transcript
from ...types import Response


def _get_kwargs(
    file_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/files/{file_id}/transcript".format(
            file_id=quote(str(file_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | Transcript:
    if response.status_code == 200:
        response_200 = Transcript.from_dict(response.json())

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
) -> Response[Problem | Transcript]:
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
) -> Response[Problem | Transcript]:
    """Read the transcript of a voice message

     409 `file_not_ready` or `file_blocked` as for `downloadFile`; without read access always 404 (12,
    9.3).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Transcript]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | Transcript | None:
    """Read the transcript of a voice message

     409 `file_not_ready` or `file_blocked` as for `downloadFile`; without read access always 404 (12,
    9.3).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Transcript
    """

    return sync_detailed(
        file_id=file_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | Transcript]:
    """Read the transcript of a voice message

     409 `file_not_ready` or `file_blocked` as for `downloadFile`; without read access always 404 (12,
    9.3).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Transcript]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | Transcript | None:
    """Read the transcript of a voice message

     409 `file_not_ready` or `file_blocked` as for `downloadFile`; without read access always 404 (12,
    9.3).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Transcript
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            client=client,
        )
    ).parsed
