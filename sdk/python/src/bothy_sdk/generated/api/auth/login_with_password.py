from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.password_login_request import PasswordLoginRequest
from ...models.problem import Problem
from ...models.token_response import TokenResponse
from ...types import Response


def _get_kwargs(
    *,
    body: PasswordLoginRequest,
    d_po_p: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["DPoP"] = d_po_p

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/password/login",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | TokenResponse:
    if response.status_code == 201:
        response_201 = TokenResponse.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

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
    body: PasswordLoginRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Log in with local password and TOTP

     Fallback for local accounts. 404 when `auth.local_password_enabled` is false. A new device stays
    `pending` until approved (08, 5.2). Unknown names, wrong passwords and locked accounts give the same
    `401`.

    Args:
        d_po_p (str):
        body (PasswordLoginRequest):

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
    body: PasswordLoginRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Log in with local password and TOTP

     Fallback for local accounts. 404 when `auth.local_password_enabled` is false. A new device stays
    `pending` until approved (08, 5.2). Unknown names, wrong passwords and locked accounts give the same
    `401`.

    Args:
        d_po_p (str):
        body (PasswordLoginRequest):

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
    body: PasswordLoginRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Log in with local password and TOTP

     Fallback for local accounts. 404 when `auth.local_password_enabled` is false. A new device stays
    `pending` until approved (08, 5.2). Unknown names, wrong passwords and locked accounts give the same
    `401`.

    Args:
        d_po_p (str):
        body (PasswordLoginRequest):

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
    body: PasswordLoginRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Log in with local password and TOTP

     Fallback for local accounts. 404 when `auth.local_password_enabled` is false. A new device stays
    `pending` until approved (08, 5.2). Unknown names, wrong passwords and locked accounts give the same
    `401`.

    Args:
        d_po_p (str):
        body (PasswordLoginRequest):

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
