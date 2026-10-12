from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    message_id: UUID,
    emoji: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/messages/{message_id}/reactions/{emoji}".format(
            message_id=quote(str(message_id), safe=""),
            emoji=quote(str(emoji), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Problem:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

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

    if response.status_code == 410:
        response_410 = Problem.from_dict(response.json())

        return response_410

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    message_id: UUID,
    emoji: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Any | Problem]:
    """Add own reaction

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        emoji (str): One Unicode grapheme cluster, percent-encoded.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        emoji=emoji,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    message_id: UUID,
    emoji: str,
    *,
    client: AuthenticatedClient | Client,
) -> Any | Problem | None:
    """Add own reaction

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        emoji (str): One Unicode grapheme cluster, percent-encoded.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        message_id=message_id,
        emoji=emoji,
        client=client,
    ).parsed


async def asyncio_detailed(
    message_id: UUID,
    emoji: str,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Any | Problem]:
    """Add own reaction

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        emoji (str): One Unicode grapheme cluster, percent-encoded.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        emoji=emoji,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    message_id: UUID,
    emoji: str,
    *,
    client: AuthenticatedClient | Client,
) -> Any | Problem | None:
    """Add own reaction

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        emoji (str): One Unicode grapheme cluster, percent-encoded.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            message_id=message_id,
            emoji=emoji,
            client=client,
        )
    ).parsed
