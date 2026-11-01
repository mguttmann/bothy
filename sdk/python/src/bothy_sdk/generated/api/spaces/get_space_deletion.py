from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.deletion_info import DeletionInfo
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    space_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/spaces/{space_id}/deletion".format(
            space_id=quote(str(space_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> DeletionInfo | Problem:
    if response.status_code == 200:
        response_200 = DeletionInfo.from_dict(response.json())

        return response_200

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
) -> Response[DeletionInfo | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[DeletionInfo | Problem]:
    """What deleting a space removes, or the state of its deletion

     Before a deletion, the counts of what it removes; after it, `state` `pending` until `undo_until`,
    then `final`. Read with `space.read`, also for a deleted space.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[DeletionInfo | Problem]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> DeletionInfo | Problem | None:
    """What deleting a space removes, or the state of its deletion

     Before a deletion, the counts of what it removes; after it, `state` `pending` until `undo_until`,
    then `final`. Read with `space.read`, also for a deleted space.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        DeletionInfo | Problem
    """

    return sync_detailed(
        space_id=space_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[DeletionInfo | Problem]:
    """What deleting a space removes, or the state of its deletion

     Before a deletion, the counts of what it removes; after it, `state` `pending` until `undo_until`,
    then `final`. Read with `space.read`, also for a deleted space.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[DeletionInfo | Problem]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> DeletionInfo | Problem | None:
    """What deleting a space removes, or the state of its deletion

     Before a deletion, the counts of what it removes; after it, `state` `pending` until `undo_until`,
    then `final`. Read with `space.read`, also for a deleted space.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        DeletionInfo | Problem
    """

    return (
        await asyncio_detailed(
            space_id=space_id,
            client=client,
        )
    ).parsed
