from http import HTTPStatus
from typing import Any, cast

import httpx

from ...client import AuthenticatedClient, Client
from ...models.client_metrics_batch import ClientMetricsBatch
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: ClientMetricsBatch,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/client-metrics",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Problem:
    if response.status_code == 202:
        response_202 = cast(Any, None)
        return response_202

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ClientMetricsBatch,
) -> Response[Any | Problem]:
    """Submit anonymous client measurements

     Only when `privacy.client_metrics` is on; otherwise `403 feature_disabled`. At most once per
    `client.metrics.flush_interval` (15, section 17).

    Args:
        body (ClientMetricsBatch): Anonymous client measurements (15, section 17). No contents, no
            IDs of threads, channels or people.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: ClientMetricsBatch,
) -> Any | Problem | None:
    """Submit anonymous client measurements

     Only when `privacy.client_metrics` is on; otherwise `403 feature_disabled`. At most once per
    `client.metrics.flush_interval` (15, section 17).

    Args:
        body (ClientMetricsBatch): Anonymous client measurements (15, section 17). No contents, no
            IDs of threads, channels or people.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ClientMetricsBatch,
) -> Response[Any | Problem]:
    """Submit anonymous client measurements

     Only when `privacy.client_metrics` is on; otherwise `403 feature_disabled`. At most once per
    `client.metrics.flush_interval` (15, section 17).

    Args:
        body (ClientMetricsBatch): Anonymous client measurements (15, section 17). No contents, no
            IDs of threads, channels or people.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: ClientMetricsBatch,
) -> Any | Problem | None:
    """Submit anonymous client measurements

     Only when `privacy.client_metrics` is on; otherwise `403 feature_disabled`. At most once per
    `client.metrics.flush_interval` (15, section 17).

    Args:
        body (ClientMetricsBatch): Anonymous client measurements (15, section 17). No contents, no
            IDs of threads, channels or people.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
