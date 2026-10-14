import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.stats_backend_report import StatsBackendReport
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    from_: datetime.datetime,
    to: datetime.datetime,
    space_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to = to.isoformat()
    params["to"] = json_to

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/stats/backends",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | StatsBackendReport:
    if response.status_code == 200:
        response_200 = StatsBackendReport.from_dict(response.json())

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
) -> Response[Problem | StatsBackendReport]:
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
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsBackendReport]:
    """Backend policy view per agent

     Per agent: configured and reported billing mode, supported modes, switches, denied invocations (I2,
    I3), cost per mode and the rate-limit state reported by the adapter (docs/spec/25-visualization.md,
    section 6.2). Agents of channels visible to the caller; cost members only with
    `cost.read.aggregate`. Never credentials, subscriptions or runtime names of an instance.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsBackendReport]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        space_id=space_id,
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
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsBackendReport | None:
    """Backend policy view per agent

     Per agent: configured and reported billing mode, supported modes, switches, denied invocations (I2,
    I3), cost per mode and the rate-limit state reported by the adapter (docs/spec/25-visualization.md,
    section 6.2). Agents of channels visible to the caller; cost members only with
    `cost.read.aggregate`. Never credentials, subscriptions or runtime names of an instance.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsBackendReport
    """

    return sync_detailed(
        client=client,
        from_=from_,
        to=to,
        space_id=space_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsBackendReport]:
    """Backend policy view per agent

     Per agent: configured and reported billing mode, supported modes, switches, denied invocations (I2,
    I3), cost per mode and the rate-limit state reported by the adapter (docs/spec/25-visualization.md,
    section 6.2). Agents of channels visible to the caller; cost members only with
    `cost.read.aggregate`. Never credentials, subscriptions or runtime names of an instance.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsBackendReport]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        space_id=space_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsBackendReport | None:
    """Backend policy view per agent

     Per agent: configured and reported billing mode, supported modes, switches, denied invocations (I2,
    I3), cost per mode and the rate-limit state reported by the adapter (docs/spec/25-visualization.md,
    section 6.2). Agents of channels visible to the caller; cost members only with
    `cost.read.aggregate`. Never credentials, subscriptions or runtime names of an instance.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsBackendReport
    """

    return (
        await asyncio_detailed(
            client=client,
            from_=from_,
            to=to,
            space_id=space_id,
        )
    ).parsed
