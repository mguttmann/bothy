from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.token_refresh_request import TokenRefreshRequest
from ...models.token_response import TokenResponse
from ...types import Response


def _get_kwargs(
    *,
    body: TokenRefreshRequest,
    d_po_p: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["DPoP"] = d_po_p

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/token/refresh",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TokenResponse:
    if response.status_code == 200:
        response_200 = TokenResponse.from_dict(response.json())

        return response_200

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
) -> Response[Problem | TokenResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: TokenRefreshRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Rotate the refresh token

     The DPoP proof must use the device key of the session. Reuse of an already rotated token revokes
    the token family and the session (`session_revoked`), except within `auth.refresh_reuse_grace`
    with the same key. In maintenance mode (`server.maintenance_mode = read_only`, ADR 0065) the
    server issues a new access token and returns the same refresh token, without rotation.

    Args:
        d_po_p (str):
        body (TokenRefreshRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TokenResponse]
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
    body: TokenRefreshRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Rotate the refresh token

     The DPoP proof must use the device key of the session. Reuse of an already rotated token revokes
    the token family and the session (`session_revoked`), except within `auth.refresh_reuse_grace`
    with the same key. In maintenance mode (`server.maintenance_mode = read_only`, ADR 0065) the
    server issues a new access token and returns the same refresh token, without rotation.

    Args:
        d_po_p (str):
        body (TokenRefreshRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TokenResponse
    """

    return sync_detailed(
        client=client,
        body=body,
        d_po_p=d_po_p,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: TokenRefreshRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Rotate the refresh token

     The DPoP proof must use the device key of the session. Reuse of an already rotated token revokes
    the token family and the session (`session_revoked`), except within `auth.refresh_reuse_grace`
    with the same key. In maintenance mode (`server.maintenance_mode = read_only`, ADR 0065) the
    server issues a new access token and returns the same refresh token, without rotation.

    Args:
        d_po_p (str):
        body (TokenRefreshRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | TokenResponse]
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
    body: TokenRefreshRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Rotate the refresh token

     The DPoP proof must use the device key of the session. Reuse of an already rotated token revokes
    the token family and the session (`session_revoked`), except within `auth.refresh_reuse_grace`
    with the same key. In maintenance mode (`server.maintenance_mode = read_only`, ADR 0065) the
    server issues a new access token and returns the same refresh token, without rotation.

    Args:
        d_po_p (str):
        body (TokenRefreshRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | TokenResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            d_po_p=d_po_p,
        )
    ).parsed
