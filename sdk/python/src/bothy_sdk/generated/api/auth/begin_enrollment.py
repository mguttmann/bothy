from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.enroll_begin_request import EnrollBeginRequest
from ...models.problem import Problem
from ...models.web_authn_challenge import WebAuthnChallenge
from ...types import Response


def _get_kwargs(
    *,
    body: EnrollBeginRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/enroll/begin",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | WebAuthnChallenge:
    if response.status_code == 200:
        response_200 = WebAuthnChallenge.from_dict(response.json())

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
) -> Response[Problem | WebAuthnChallenge]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: EnrollBeginRequest,
) -> Response[Problem | WebAuthnChallenge]:
    """Start the first passkey registration with an enrollment code

     Starts the first passkey registration with an enrollment code of purpose `first_login` or
    `recovery`. The code is checked but not redeemed; the stored challenge keeps a reference to it.
    Every rejected code gives `401`.

    Args:
        body (EnrollBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | WebAuthnChallenge]
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
    body: EnrollBeginRequest,
) -> Problem | WebAuthnChallenge | None:
    """Start the first passkey registration with an enrollment code

     Starts the first passkey registration with an enrollment code of purpose `first_login` or
    `recovery`. The code is checked but not redeemed; the stored challenge keeps a reference to it.
    Every rejected code gives `401`.

    Args:
        body (EnrollBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | WebAuthnChallenge
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: EnrollBeginRequest,
) -> Response[Problem | WebAuthnChallenge]:
    """Start the first passkey registration with an enrollment code

     Starts the first passkey registration with an enrollment code of purpose `first_login` or
    `recovery`. The code is checked but not redeemed; the stored challenge keeps a reference to it.
    Every rejected code gives `401`.

    Args:
        body (EnrollBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | WebAuthnChallenge]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: EnrollBeginRequest,
) -> Problem | WebAuthnChallenge | None:
    """Start the first passkey registration with an enrollment code

     Starts the first passkey registration with an enrollment code of purpose `first_login` or
    `recovery`. The code is checked but not redeemed; the stored challenge keeps a reference to it.
    Every rejected code gives `401`.

    Args:
        body (EnrollBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | WebAuthnChallenge
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
