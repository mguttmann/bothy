from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...models.event_page import EventPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    consumer: str,
    *,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/me/consumers/{consumer}/events".format(
            consumer=quote(str(consumer), safe=""),
        ),
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
) -> Response[EventPage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> Response[EventPage | Problem]:
    """Fetch events for a consumer without WebSocket

     Delivers from `acked_seq + 1`. Invocation events carry a fresh `invocation_token`.
    Unacknowledged events are delivered again after `agent.ack_timeout`.

    Args:
        consumer (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EventPage | Problem]
    """

    kwargs = _get_kwargs(
        consumer=consumer,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> EventPage | Problem | None:
    """Fetch events for a consumer without WebSocket

     Delivers from `acked_seq + 1`. Invocation events carry a fresh `invocation_token`.
    Unacknowledged events are delivered again after `agent.ack_timeout`.

    Args:
        consumer (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EventPage | Problem
    """

    return sync_detailed(
        consumer=consumer,
        client=client,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> Response[EventPage | Problem]:
    """Fetch events for a consumer without WebSocket

     Delivers from `acked_seq + 1`. Invocation events carry a fresh `invocation_token`.
    Unacknowledged events are delivered again after `agent.ack_timeout`.

    Args:
        consumer (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EventPage | Problem]
    """

    kwargs = _get_kwargs(
        consumer=consumer,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
) -> EventPage | Problem | None:
    """Fetch events for a consumer without WebSocket

     Delivers from `acked_seq + 1`. Invocation events carry a fresh `invocation_token`.
    Unacknowledged events are delivered again after `agent.ack_timeout`.

    Args:
        consumer (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EventPage | Problem
    """

    return (
        await asyncio_detailed(
            consumer=consumer,
            client=client,
            limit=limit,
        )
    ).parsed
