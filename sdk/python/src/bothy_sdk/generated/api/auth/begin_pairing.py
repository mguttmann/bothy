from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.pairing_begin_request import PairingBeginRequest
from ...models.pairing_started import PairingStarted
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: PairingBeginRequest,
    d_po_p: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["DPoP"] = d_po_p

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/pairing/begin",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> PairingStarted | Problem:
    if response.status_code == 201:
        response_201 = PairingStarted.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[PairingStarted | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PairingBeginRequest,
    d_po_p: str,
) -> Response[PairingStarted | Problem]:
    """Start a device pairing without login

     The new device registers its public keys and shows the returned code. An active device of the same
    person approves it with `approvePairing` (08, 5.3).

    Args:
        d_po_p (str):
        body (PairingBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PairingStarted | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        d_po_p=d_po_p,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: PairingBeginRequest,
    d_po_p: str,
) -> PairingStarted | Problem | None:
    """Start a device pairing without login

     The new device registers its public keys and shows the returned code. An active device of the same
    person approves it with `approvePairing` (08, 5.3).

    Args:
        d_po_p (str):
        body (PairingBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PairingStarted | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        d_po_p=d_po_p,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PairingBeginRequest,
    d_po_p: str,
) -> Response[PairingStarted | Problem]:
    """Start a device pairing without login

     The new device registers its public keys and shows the returned code. An active device of the same
    person approves it with `approvePairing` (08, 5.3).

    Args:
        d_po_p (str):
        body (PairingBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PairingStarted | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        d_po_p=d_po_p,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: PairingBeginRequest,
    d_po_p: str,
) -> PairingStarted | Problem | None:
    """Start a device pairing without login

     The new device registers its public keys and shows the returned code. An active device of the same
    person approves it with `approvePairing` (08, 5.3).

    Args:
        d_po_p (str):
        body (PairingBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PairingStarted | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            d_po_p=d_po_p,
        )
    ).parsed
