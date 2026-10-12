from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.forum_tag import ForumTag
from ...models.move_position import MovePosition
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    channel_id: UUID,
    tag_id: UUID,
    *,
    body: MovePosition,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/channels/{channel_id}/tags/{tag_id}/move".format(
            channel_id=quote(str(channel_id), safe=""),
            tag_id=quote(str(tag_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ForumTag | Problem:
    if response.status_code == 200:
        response_200 = ForumTag.from_dict(response.json())

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

    if response.status_code == 422:
        response_422 = Problem.from_dict(response.json())

        return response_422

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ForumTag | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    tag_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MovePosition,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ForumTag | Problem]:
    """Change the position of a forum tag

     Sets `sort_key` relative to sibling tags of the same forum; bumps the channel version and writes
    `channel.updated` like every tag change (03 `forum_tags`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        tag_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MovePosition): New position relative to siblings. Exactly one of both fields; none
            moves to the start.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ForumTag | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        tag_id=tag_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    tag_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MovePosition,
    idempotency_key: UUID | Unset = UNSET,
) -> ForumTag | Problem | None:
    """Change the position of a forum tag

     Sets `sort_key` relative to sibling tags of the same forum; bumps the channel version and writes
    `channel.updated` like every tag change (03 `forum_tags`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        tag_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MovePosition): New position relative to siblings. Exactly one of both fields; none
            moves to the start.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ForumTag | Problem
    """

    return sync_detailed(
        channel_id=channel_id,
        tag_id=tag_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    tag_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MovePosition,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ForumTag | Problem]:
    """Change the position of a forum tag

     Sets `sort_key` relative to sibling tags of the same forum; bumps the channel version and writes
    `channel.updated` like every tag change (03 `forum_tags`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        tag_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MovePosition): New position relative to siblings. Exactly one of both fields; none
            moves to the start.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ForumTag | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        tag_id=tag_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    tag_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MovePosition,
    idempotency_key: UUID | Unset = UNSET,
) -> ForumTag | Problem | None:
    """Change the position of a forum tag

     Sets `sort_key` relative to sibling tags of the same forum; bumps the channel version and writes
    `channel.updated` like every tag change (03 `forum_tags`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        tag_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MovePosition): New position relative to siblings. Exactly one of both fields; none
            moves to the start.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ForumTag | Problem
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            tag_id=tag_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
