from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...models.consumer_ack import ConsumerAck
from ...models.consumer_state import ConsumerState
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    consumer: str,
    *,
    body: ConsumerAck,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/agents/me/consumers/{consumer}/ack".format(
            consumer=quote(str(consumer), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ConsumerState | Problem:
    if response.status_code == 200:
        response_200 = ConsumerState.from_dict(response.json())

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
) -> Response[ConsumerState | Problem]:
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
    body: ConsumerAck,
) -> Response[ConsumerState | Problem]:
    """Acknowledge events up to a sequence

    Args:
        consumer (str):
        body (ConsumerAck):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConsumerState | Problem]
    """

    kwargs = _get_kwargs(
        consumer=consumer,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    body: ConsumerAck,
) -> ConsumerState | Problem | None:
    """Acknowledge events up to a sequence

    Args:
        consumer (str):
        body (ConsumerAck):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConsumerState | Problem
    """

    return sync_detailed(
        consumer=consumer,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    body: ConsumerAck,
) -> Response[ConsumerState | Problem]:
    """Acknowledge events up to a sequence

    Args:
        consumer (str):
        body (ConsumerAck):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConsumerState | Problem]
    """

    kwargs = _get_kwargs(
        consumer=consumer,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    consumer: str,
    *,
    client: AuthenticatedClient | Client,
    body: ConsumerAck,
) -> ConsumerState | Problem | None:
    """Acknowledge events up to a sequence

    Args:
        consumer (str):
        body (ConsumerAck):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConsumerState | Problem
    """

    return (
        await asyncio_detailed(
            consumer=consumer,
            client=client,
            body=body,
        )
    ).parsed
