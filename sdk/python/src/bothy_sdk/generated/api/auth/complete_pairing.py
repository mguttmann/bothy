from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.pairing_complete_request import PairingCompleteRequest
from ...models.pairing_status import PairingStatus
from ...models.problem import Problem
from ...models.token_response import TokenResponse
from ...types import Response


def _get_kwargs(
    *,
    body: PairingCompleteRequest,
    d_po_p: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["DPoP"] = d_po_p

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/pairing/complete",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> PairingStatus | Problem | TokenResponse:
    if response.status_code == 201:
        response_201 = TokenResponse.from_dict(response.json())

        return response_201

    if response.status_code == 202:
        response_202 = PairingStatus.from_dict(response.json())

        return response_202

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 410:
        response_410 = Problem.from_dict(response.json())

        return response_410

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[PairingStatus | Problem | TokenResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PairingCompleteRequest,
    d_po_p: str,
) -> Response[PairingStatus | Problem | TokenResponse]:
    """Collect the tokens of an approved pairing

     The DPoP proof must be signed with the `dpop` key registered at `beginPairing`; a proof signed by
    another key rejects the pairing. Before approval the answer is `202` with the status; after expiry
    or rejection `410`.

    Args:
        d_po_p (str):
        body (PairingCompleteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PairingStatus | Problem | TokenResponse]
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
    body: PairingCompleteRequest,
    d_po_p: str,
) -> PairingStatus | Problem | TokenResponse | None:
    """Collect the tokens of an approved pairing

     The DPoP proof must be signed with the `dpop` key registered at `beginPairing`; a proof signed by
    another key rejects the pairing. Before approval the answer is `202` with the status; after expiry
    or rejection `410`.

    Args:
        d_po_p (str):
        body (PairingCompleteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PairingStatus | Problem | TokenResponse
    """

    return sync_detailed(
        client=client,
        body=body,
        d_po_p=d_po_p,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PairingCompleteRequest,
    d_po_p: str,
) -> Response[PairingStatus | Problem | TokenResponse]:
    """Collect the tokens of an approved pairing

     The DPoP proof must be signed with the `dpop` key registered at `beginPairing`; a proof signed by
    another key rejects the pairing. Before approval the answer is `202` with the status; after expiry
    or rejection `410`.

    Args:
        d_po_p (str):
        body (PairingCompleteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PairingStatus | Problem | TokenResponse]
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
    body: PairingCompleteRequest,
    d_po_p: str,
) -> PairingStatus | Problem | TokenResponse | None:
    """Collect the tokens of an approved pairing

     The DPoP proof must be signed with the `dpop` key registered at `beginPairing`; a proof signed by
    another key rejects the pairing. Before approval the answer is `202` with the status; after expiry
    or rejection `410`.

    Args:
        d_po_p (str):
        body (PairingCompleteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PairingStatus | Problem | TokenResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            d_po_p=d_po_p,
        )
    ).parsed
