import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.cost_series import CostSeries
from ...models.get_cost_stats_group_by import GetCostStatsGroupBy
from ...models.problem import Problem
from ...models.stats_granularity import StatsGranularity
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    group_by: GetCostStatsGroupBy | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to = to.isoformat()
    params["to"] = json_to

    json_granularity: str | Unset = UNSET
    if not isinstance(granularity, Unset):
        json_granularity = granularity.value

    params["granularity"] = json_granularity

    json_group_by: str | Unset = UNSET
    if not isinstance(group_by, Unset):
        json_group_by = group_by.value

    params["group_by"] = json_group_by

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    json_agent_id: str | Unset = UNSET
    if not isinstance(agent_id, Unset):
        json_agent_id = str(agent_id)
    params["agent_id"] = json_agent_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/stats/costs",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CostSeries | Problem:
    if response.status_code == 200:
        response_200 = CostSeries.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[CostSeries | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    group_by: GetCostStatsGroupBy | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Response[CostSeries | Problem]:
    """Costs per space, agent or instance

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        group_by (GetCostStatsGroupBy | Unset):
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostSeries | Problem]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        granularity=granularity,
        group_by=group_by,
        space_id=space_id,
        agent_id=agent_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    group_by: GetCostStatsGroupBy | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> CostSeries | Problem | None:
    """Costs per space, agent or instance

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        group_by (GetCostStatsGroupBy | Unset):
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostSeries | Problem
    """

    return sync_detailed(
        client=client,
        from_=from_,
        to=to,
        granularity=granularity,
        group_by=group_by,
        space_id=space_id,
        agent_id=agent_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    group_by: GetCostStatsGroupBy | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> Response[CostSeries | Problem]:
    """Costs per space, agent or instance

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        group_by (GetCostStatsGroupBy | Unset):
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostSeries | Problem]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        granularity=granularity,
        group_by=group_by,
        space_id=space_id,
        agent_id=agent_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    group_by: GetCostStatsGroupBy | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
) -> CostSeries | Problem | None:
    """Costs per space, agent or instance

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        group_by (GetCostStatsGroupBy | Unset):
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostSeries | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            from_=from_,
            to=to,
            granularity=granularity,
            group_by=group_by,
            space_id=space_id,
            agent_id=agent_id,
        )
    ).parsed
