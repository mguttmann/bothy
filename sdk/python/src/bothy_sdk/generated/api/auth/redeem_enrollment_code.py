from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.enroll_redeem_request import EnrollRedeemRequest
from ...models.problem import Problem
from ...models.token_response import TokenResponse
from ...types import Response


def _get_kwargs(
    *,
    body: EnrollRedeemRequest,
    d_po_p: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["DPoP"] = d_po_p

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/enroll/redeem",
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
    body: EnrollRedeemRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Redeem an enrollment code with device keys and log in

     Redeems an enrollment code without a passkey; the redeeming device becomes `active` (08, 3.2). Every
    rejected code (unknown, used, expired, exhausted, wrong purpose, person not active) gets the same
    `401` body. Code and device keys are checked in one transaction; a failed attempt with a matching
    code counts against `auth.enrollment_code_max_attempts`.

    Args:
        d_po_p (str):
        body (EnrollRedeemRequest): Redeems an enrollment code with device keys only; the
            redeeming device becomes `active` (08, 3.2).

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
    body: EnrollRedeemRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Redeem an enrollment code with device keys and log in

     Redeems an enrollment code without a passkey; the redeeming device becomes `active` (08, 3.2). Every
    rejected code (unknown, used, expired, exhausted, wrong purpose, person not active) gets the same
    `401` body. Code and device keys are checked in one transaction; a failed attempt with a matching
    code counts against `auth.enrollment_code_max_attempts`.

    Args:
        d_po_p (str):
        body (EnrollRedeemRequest): Redeems an enrollment code with device keys only; the
            redeeming device becomes `active` (08, 3.2).

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
    body: EnrollRedeemRequest,
    d_po_p: str,
) -> Response[Problem | TokenResponse]:
    """Redeem an enrollment code with device keys and log in

     Redeems an enrollment code without a passkey; the redeeming device becomes `active` (08, 3.2). Every
    rejected code (unknown, used, expired, exhausted, wrong purpose, person not active) gets the same
    `401` body. Code and device keys are checked in one transaction; a failed attempt with a matching
    code counts against `auth.enrollment_code_max_attempts`.

    Args:
        d_po_p (str):
        body (EnrollRedeemRequest): Redeems an enrollment code with device keys only; the
            redeeming device becomes `active` (08, 3.2).

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
    body: EnrollRedeemRequest,
    d_po_p: str,
) -> Problem | TokenResponse | None:
    """Redeem an enrollment code with device keys and log in

     Redeems an enrollment code without a passkey; the redeeming device becomes `active` (08, 3.2). Every
    rejected code (unknown, used, expired, exhausted, wrong purpose, person not active) gets the same
    `401` body. Code and device keys are checked in one transaction; a failed attempt with a matching
    code counts against `auth.enrollment_code_max_attempts`.

    Args:
        d_po_p (str):
        body (EnrollRedeemRequest): Redeems an enrollment code with device keys only; the
            redeeming device becomes `active` (08, 3.2).

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
