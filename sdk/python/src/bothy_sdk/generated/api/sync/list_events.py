from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.event_page import EventPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    after: int,
    epoch: UUID | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["after"] = after

    json_epoch: str | Unset = UNSET
    if not isinstance(epoch, Unset):
        json_epoch = str(epoch)
    params["epoch"] = json_epoch

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/events",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> EventPage | Problem:
    if response.status_code == 200:
        response_200 = EventPage.from_dict(response.json())

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
) -> Response[EventPage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    after: int,
    epoch: UUID | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[EventPage | Problem]:
    """Catch up on events after a sequence

     Same filtering as the gateway. 410 `resync_required` with `reason` when catch-up is impossible.

    Args:
        after (int):
        epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EventPage | Problem]
    """

    kwargs = _get_kwargs(
        after=after,
        epoch=epoch,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    after: int,
    epoch: UUID | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> EventPage | Problem | None:
    """Catch up on events after a sequence

     Same filtering as the gateway. 410 `resync_required` with `reason` when catch-up is impossible.

    Args:
        after (int):
        epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EventPage | Problem
    """

    return sync_detailed(
        client=client,
        after=after,
        epoch=epoch,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    after: int,
    epoch: UUID | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[EventPage | Problem]:
    """Catch up on events after a sequence

     Same filtering as the gateway. 410 `resync_required` with `reason` when catch-up is impossible.

    Args:
        after (int):
        epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EventPage | Problem]
    """

    kwargs = _get_kwargs(
        after=after,
        epoch=epoch,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    after: int,
    epoch: UUID | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> EventPage | Problem | None:
    """Catch up on events after a sequence

     Same filtering as the gateway. 410 `resync_required` with `reason` when catch-up is impossible.

    Args:
        after (int):
        epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EventPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            after=after,
            epoch=epoch,
            limit=limit,
        )
    ).parsed
