from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.host_pair_request import HostPairRequest
from ...models.host_pair_result import HostPairResult
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: HostPairRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hosts/pair",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> HostPairResult | Problem:
    if response.status_code == 201:
        response_201 = HostPairResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[HostPairResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: HostPairRequest,
) -> Response[HostPairResult | Problem]:
    """Redeem a pairing code from the host service

     Called by `bothy-host pair` without login. Registers the static X25519 key (Noise) and the Ed25519
    key (login) of the host, its profiles and the names of its session users (`reported_session_users`,
    24 section 2.2). The host becomes `confirming`; an owner confirms the fingerprint (24, 3.2).

    Args:
        body (HostPairRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[HostPairResult | Problem]
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
    body: HostPairRequest,
) -> HostPairResult | Problem | None:
    """Redeem a pairing code from the host service

     Called by `bothy-host pair` without login. Registers the static X25519 key (Noise) and the Ed25519
    key (login) of the host, its profiles and the names of its session users (`reported_session_users`,
    24 section 2.2). The host becomes `confirming`; an owner confirms the fingerprint (24, 3.2).

    Args:
        body (HostPairRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        HostPairResult | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: HostPairRequest,
) -> Response[HostPairResult | Problem]:
    """Redeem a pairing code from the host service

     Called by `bothy-host pair` without login. Registers the static X25519 key (Noise) and the Ed25519
    key (login) of the host, its profiles and the names of its session users (`reported_session_users`,
    24 section 2.2). The host becomes `confirming`; an owner confirms the fingerprint (24, 3.2).

    Args:
        body (HostPairRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[HostPairResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: HostPairRequest,
) -> HostPairResult | Problem | None:
    """Redeem a pairing code from the host service

     Called by `bothy-host pair` without login. Registers the static X25519 key (Noise) and the Ed25519
    key (login) of the host, its profiles and the names of its session users (`reported_session_users`,
    24 section 2.2). The host becomes `confirming`; an owner confirms the fingerprint (24, 3.2).

    Args:
        body (HostPairRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        HostPairResult | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
