from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.grant_create import GrantCreate
from ...models.grant_preview import GrantPreview
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: GrantCreate,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/grants/preview",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GrantPreview | Problem:
    if response.status_code == 200:
        response_200 = GrantPreview.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[GrantPreview | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: GrantCreate,
) -> Response[GrantPreview | Problem]:
    """Show what a grant would change

     Dry run of `createGrant` (action `access.preview`): affected principals, level changes and
    invariant violations. No step-up needed; the transaction is rolled back, nothing is stored.
    The caller needs read access to the target (else `404`) and `owner` or `manage` on it (else
    `403`); limits of delegation give `200` with `allowed: false` and `denial_reason`. `text`
    is in the caller's language (`de` or `en`, else `de`) with dates in the caller's time zone
    (07, 9.3).

    Args:
        body (GrantCreate): `level` is required for every target except `host`, where it must be
            absent and `capabilities` must be exactly `[terminal]` (07, 2.12).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GrantPreview | Problem]
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
    body: GrantCreate,
) -> GrantPreview | Problem | None:
    """Show what a grant would change

     Dry run of `createGrant` (action `access.preview`): affected principals, level changes and
    invariant violations. No step-up needed; the transaction is rolled back, nothing is stored.
    The caller needs read access to the target (else `404`) and `owner` or `manage` on it (else
    `403`); limits of delegation give `200` with `allowed: false` and `denial_reason`. `text`
    is in the caller's language (`de` or `en`, else `de`) with dates in the caller's time zone
    (07, 9.3).

    Args:
        body (GrantCreate): `level` is required for every target except `host`, where it must be
            absent and `capabilities` must be exactly `[terminal]` (07, 2.12).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GrantPreview | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: GrantCreate,
) -> Response[GrantPreview | Problem]:
    """Show what a grant would change

     Dry run of `createGrant` (action `access.preview`): affected principals, level changes and
    invariant violations. No step-up needed; the transaction is rolled back, nothing is stored.
    The caller needs read access to the target (else `404`) and `owner` or `manage` on it (else
    `403`); limits of delegation give `200` with `allowed: false` and `denial_reason`. `text`
    is in the caller's language (`de` or `en`, else `de`) with dates in the caller's time zone
    (07, 9.3).

    Args:
        body (GrantCreate): `level` is required for every target except `host`, where it must be
            absent and `capabilities` must be exactly `[terminal]` (07, 2.12).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GrantPreview | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: GrantCreate,
) -> GrantPreview | Problem | None:
    """Show what a grant would change

     Dry run of `createGrant` (action `access.preview`): affected principals, level changes and
    invariant violations. No step-up needed; the transaction is rolled back, nothing is stored.
    The caller needs read access to the target (else `404`) and `owner` or `manage` on it (else
    `403`); limits of delegation give `200` with `allowed: false` and `denial_reason`. `text`
    is in the caller's language (`de` or `en`, else `de`) with dates in the caller's time zone
    (07, 9.3).

    Args:
        body (GrantCreate): `level` is required for every target except `host`, where it must be
            absent and `capabilities` must be exactly `[terminal]` (07, 2.12).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GrantPreview | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
