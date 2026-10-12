import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.cost_estimate_page import CostEstimatePage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    from_: datetime.datetime,
    to: datetime.datetime,
    agent_id: list[UUID],
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to = to.isoformat()
    params["to"] = json_to

    json_agent_id = []
    for agent_id_item_data in agent_id:
        agent_id_item = str(agent_id_item_data)
        json_agent_id.append(agent_id_item)

    params["agent_id"] = json_agent_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/costs/estimates",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CostEstimatePage | Problem:
    if response.status_code == 200:
        response_200 = CostEstimatePage.from_dict(response.json())

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
) -> Response[CostEstimatePage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    from_: datetime.datetime,
    to: datetime.datetime,
    agent_id: list[UUID],
) -> Response[CostEstimatePage | Problem]:
    """Cost estimates per run for reconciliation

     For the reconciliation app with scope `costs:reconcile` and for owners in a session, without step-up
    (05, R-AZ-28). One item per run of the given agents with cost entries of mode `organization` whose
    `occurred_at` lies in the range, sorted by `run_id`; runs whose entries in the range mix currencies
    are left out. Entries of mode `personal` never appear (03 `cost_entries`). Items name no channel,
    thread or content; the list is evaluated on the instance and not filtered by visible channels (06, 4
    point 2).

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        agent_id (list[UUID]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostEstimatePage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        from_=from_,
        to=to,
        agent_id=agent_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    from_: datetime.datetime,
    to: datetime.datetime,
    agent_id: list[UUID],
) -> CostEstimatePage | Problem | None:
    """Cost estimates per run for reconciliation

     For the reconciliation app with scope `costs:reconcile` and for owners in a session, without step-up
    (05, R-AZ-28). One item per run of the given agents with cost entries of mode `organization` whose
    `occurred_at` lies in the range, sorted by `run_id`; runs whose entries in the range mix currencies
    are left out. Entries of mode `personal` never appear (03 `cost_entries`). Items name no channel,
    thread or content; the list is evaluated on the instance and not filtered by visible channels (06, 4
    point 2).

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        agent_id (list[UUID]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostEstimatePage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        from_=from_,
        to=to,
        agent_id=agent_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    from_: datetime.datetime,
    to: datetime.datetime,
    agent_id: list[UUID],
) -> Response[CostEstimatePage | Problem]:
    """Cost estimates per run for reconciliation

     For the reconciliation app with scope `costs:reconcile` and for owners in a session, without step-up
    (05, R-AZ-28). One item per run of the given agents with cost entries of mode `organization` whose
    `occurred_at` lies in the range, sorted by `run_id`; runs whose entries in the range mix currencies
    are left out. Entries of mode `personal` never appear (03 `cost_entries`). Items name no channel,
    thread or content; the list is evaluated on the instance and not filtered by visible channels (06, 4
    point 2).

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        agent_id (list[UUID]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostEstimatePage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        from_=from_,
        to=to,
        agent_id=agent_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    from_: datetime.datetime,
    to: datetime.datetime,
    agent_id: list[UUID],
) -> CostEstimatePage | Problem | None:
    """Cost estimates per run for reconciliation

     For the reconciliation app with scope `costs:reconcile` and for owners in a session, without step-up
    (05, R-AZ-28). One item per run of the given agents with cost entries of mode `organization` whose
    `occurred_at` lies in the range, sorted by `run_id`; runs whose entries in the range mix currencies
    are left out. Entries of mode `personal` never appear (03 `cost_entries`). Items name no channel,
    thread or content; the list is evaluated on the instance and not filtered by visible channels (06, 4
    point 2).

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        agent_id (list[UUID]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostEstimatePage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            from_=from_,
            to=to,
            agent_id=agent_id,
        )
    ).parsed
