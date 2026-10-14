from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.stats_live_frame import StatsLiveFrame
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    space_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/stats/live",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | StatsLiveFrame:
    if response.status_code == 200:
        response_200 = StatsLiveFrame.from_dict(response.json())

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
) -> Response[Problem | StatsLiveFrame]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsLiveFrame]:
    """Current live frame

     The latest live frame (`StatsLiveFrame`) of the instance or of one space, the same frame the gateway
    sends as `stats.live` (docs/spec/25-visualization.md, section 5). The server returns the base frame
    built for the current `stats.live_interval`; polling does not rebuild it. Filtered to the channels
    visible to the caller; cost members only with `cost.read.aggregate`. `403 feature_disabled` when
    `stats.live_enabled` is off.

    Args:
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsLiveFrame]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsLiveFrame | None:
    """Current live frame

     The latest live frame (`StatsLiveFrame`) of the instance or of one space, the same frame the gateway
    sends as `stats.live` (docs/spec/25-visualization.md, section 5). The server returns the base frame
    built for the current `stats.live_interval`; polling does not rebuild it. Filtered to the channels
    visible to the caller; cost members only with `cost.read.aggregate`. `403 feature_disabled` when
    `stats.live_enabled` is off.

    Args:
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsLiveFrame
    """

    return sync_detailed(
        client=client,
        space_id=space_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsLiveFrame]:
    """Current live frame

     The latest live frame (`StatsLiveFrame`) of the instance or of one space, the same frame the gateway
    sends as `stats.live` (docs/spec/25-visualization.md, section 5). The server returns the base frame
    built for the current `stats.live_interval`; polling does not rebuild it. Filtered to the channels
    visible to the caller; cost members only with `cost.read.aggregate`. `403 feature_disabled` when
    `stats.live_enabled` is off.

    Args:
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsLiveFrame]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsLiveFrame | None:
    """Current live frame

     The latest live frame (`StatsLiveFrame`) of the instance or of one space, the same frame the gateway
    sends as `stats.live` (docs/spec/25-visualization.md, section 5). The server returns the base frame
    built for the current `stats.live_interval`; polling does not rebuild it. Filtered to the channels
    visible to the caller; cost members only with `cost.read.aggregate`. `403 feature_disabled` when
    `stats.live_enabled` is off.

    Args:
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsLiveFrame
    """

    return (
        await asyncio_detailed(
            client=client,
            space_id=space_id,
        )
    ).parsed
