from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.reaction_page import ReactionPage
from ...types import UNSET, Response, Unset


def _get_kwargs(
    message_id: UUID,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    emoji: str | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    params["emoji"] = emoji

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/messages/{message_id}/reactions".format(
            message_id=quote(str(message_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ReactionPage:
    if response.status_code == 200:
        response_200 = ReactionPage.from_dict(response.json())

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
) -> Response[Problem | ReactionPage]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    emoji: str | Unset = UNSET,
) -> Response[Problem | ReactionPage]:
    """List who reacted

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        emoji (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReactionPage]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        cursor=cursor,
        limit=limit,
        emoji=emoji,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    emoji: str | Unset = UNSET,
) -> Problem | ReactionPage | None:
    """List who reacted

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        emoji (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReactionPage
    """

    return sync_detailed(
        message_id=message_id,
        client=client,
        cursor=cursor,
        limit=limit,
        emoji=emoji,
    ).parsed


async def asyncio_detailed(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    emoji: str | Unset = UNSET,
) -> Response[Problem | ReactionPage]:
    """List who reacted

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        emoji (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ReactionPage]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        cursor=cursor,
        limit=limit,
        emoji=emoji,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    emoji: str | Unset = UNSET,
) -> Problem | ReactionPage | None:
    """List who reacted

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        emoji (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ReactionPage
    """

    return (
        await asyncio_detailed(
            message_id=message_id,
            client=client,
            cursor=cursor,
            limit=limit,
            emoji=emoji,
        )
    ).parsed
