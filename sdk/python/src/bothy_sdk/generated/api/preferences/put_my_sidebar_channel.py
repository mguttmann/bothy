from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.sidebar_channel_put import SidebarChannelPut
from ...models.sidebar_channel_state import SidebarChannelState
from ...types import Response


def _get_kwargs(
    channel_id: UUID,
    *,
    body: SidebarChannelPut,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/sidebar/channels/{channel_id}".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | SidebarChannelState:
    if response.status_code == 200:
        response_200 = SidebarChannelState.from_dict(response.json())

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
) -> Response[Problem | SidebarChannelState]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarChannelPut,
) -> Response[Problem | SidebarChannelState]:
    """Reorder the posts of a channel for oneself

     Stores the caller's personal order of the posts of a channel the caller can read, version + 1, last
    write wins, no If-Match; an empty list returns to the default order. The server checks the form of
    the list (unique items, at most `sidebar.order_max_items`), not whether each ID names a thread the
    caller can see. A channel the caller cannot read and an unknown one give the same 404. Writes
    `sidebar_channel.updated` for the caller.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarChannelPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SidebarChannelState]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarChannelPut,
) -> Problem | SidebarChannelState | None:
    """Reorder the posts of a channel for oneself

     Stores the caller's personal order of the posts of a channel the caller can read, version + 1, last
    write wins, no If-Match; an empty list returns to the default order. The server checks the form of
    the list (unique items, at most `sidebar.order_max_items`), not whether each ID names a thread the
    caller can see. A channel the caller cannot read and an unknown one give the same 404. Writes
    `sidebar_channel.updated` for the caller.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarChannelPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SidebarChannelState
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarChannelPut,
) -> Response[Problem | SidebarChannelState]:
    """Reorder the posts of a channel for oneself

     Stores the caller's personal order of the posts of a channel the caller can read, version + 1, last
    write wins, no If-Match; an empty list returns to the default order. The server checks the form of
    the list (unique items, at most `sidebar.order_max_items`), not whether each ID names a thread the
    caller can see. A channel the caller cannot read and an unknown one give the same 404. Writes
    `sidebar_channel.updated` for the caller.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarChannelPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SidebarChannelState]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarChannelPut,
) -> Problem | SidebarChannelState | None:
    """Reorder the posts of a channel for oneself

     Stores the caller's personal order of the posts of a channel the caller can read, version + 1, last
    write wins, no If-Match; an empty list returns to the default order. The server checks the form of
    the list (unique items, at most `sidebar.order_max_items`), not whether each ID names a thread the
    caller can see. A channel the caller cannot read and an unknown one give the same 404. Writes
    `sidebar_channel.updated` for the caller.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarChannelPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SidebarChannelState
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
            body=body,
        )
    ).parsed
