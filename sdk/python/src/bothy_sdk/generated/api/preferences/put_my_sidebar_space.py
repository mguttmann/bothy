from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.sidebar_state import SidebarState
from ...models.sidebar_state_put import SidebarStatePut
from ...types import Response


def _get_kwargs(
    space_id: UUID,
    *,
    body: SidebarStatePut,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/sidebar/spaces/{space_id}".format(
            space_id=quote(str(space_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | SidebarState:
    if response.status_code == 200:
        response_200 = SidebarState.from_dict(response.json())

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
) -> Response[Problem | SidebarState]:
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
    body: SidebarStatePut,
) -> Response[Problem | SidebarState]:
    """Collapse or reorder a space for oneself

     Stores the caller's row of the space, version + 1, last write wins, no If-Match. `channel_order`
    absent keeps the stored channel order, a list replaces it and an empty list returns to the default
    order. The server checks the form of the list (unique items, at most `sidebar.order_max_items`), not
    whether each ID names a channel the caller can see. Writes `sidebar.updated` for the caller.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarStatePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SidebarState]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarStatePut,
) -> Problem | SidebarState | None:
    """Collapse or reorder a space for oneself

     Stores the caller's row of the space, version + 1, last write wins, no If-Match. `channel_order`
    absent keeps the stored channel order, a list replaces it and an empty list returns to the default
    order. The server checks the form of the list (unique items, at most `sidebar.order_max_items`), not
    whether each ID names a channel the caller can see. Writes `sidebar.updated` for the caller.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarStatePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SidebarState
    """

    return sync_detailed(
        space_id=space_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarStatePut,
) -> Response[Problem | SidebarState]:
    """Collapse or reorder a space for oneself

     Stores the caller's row of the space, version + 1, last write wins, no If-Match. `channel_order`
    absent keeps the stored channel order, a list replaces it and an empty list returns to the default
    order. The server checks the form of the list (unique items, at most `sidebar.order_max_items`), not
    whether each ID names a channel the caller can see. Writes `sidebar.updated` for the caller.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarStatePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SidebarState]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SidebarStatePut,
) -> Problem | SidebarState | None:
    """Collapse or reorder a space for oneself

     Stores the caller's row of the space, version + 1, last write wins, no If-Match. `channel_order`
    absent keeps the stored channel order, a list replaces it and an empty list returns to the default
    order. The server checks the form of the list (unique items, at most `sidebar.order_max_items`), not
    whether each ID names a channel the caller can see. Writes `sidebar.updated` for the caller.

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SidebarStatePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SidebarState
    """

    return (
        await asyncio_detailed(
            space_id=space_id,
            client=client,
            body=body,
        )
    ).parsed
