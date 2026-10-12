from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.stats_slo_report import StatsSloReport
from ...types import Response


def _get_kwargs() -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/stats/slo",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | StatsSloReport:
    if response.status_code == 200:
        response_200 = StatsSloReport.from_dict(response.json())

        return response_200

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
) -> Response[Problem | StatsSloReport]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | StatsSloReport]:
    """SLO traffic light from own measurement

     One entry per SLO of the Sloth files, computed from `stats_minutes` over the SLO period
    (docs/spec/25-visualisierung.md, section 6.4). Label "own measurement": values may differ from
    Prometheus. Requires `stats.read` on the instance (owner); a space manager gets `403 forbidden`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsSloReport]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | StatsSloReport | None:
    """SLO traffic light from own measurement

     One entry per SLO of the Sloth files, computed from `stats_minutes` over the SLO period
    (docs/spec/25-visualisierung.md, section 6.4). Label "own measurement": values may differ from
    Prometheus. Requires `stats.read` on the instance (owner); a space manager gets `403 forbidden`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsSloReport
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | StatsSloReport]:
    """SLO traffic light from own measurement

     One entry per SLO of the Sloth files, computed from `stats_minutes` over the SLO period
    (docs/spec/25-visualisierung.md, section 6.4). Label "own measurement": values may differ from
    Prometheus. Requires `stats.read` on the instance (owner); a space manager gets `403 forbidden`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsSloReport]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> Problem | StatsSloReport | None:
    """SLO traffic light from own measurement

     One entry per SLO of the Sloth files, computed from `stats_minutes` over the SLO period
    (docs/spec/25-visualisierung.md, section 6.4). Label "own measurement": values may differ from
    Prometheus. Requires `stats.read` on the instance (owner); a space manager gets `403 forbidden`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsSloReport
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
