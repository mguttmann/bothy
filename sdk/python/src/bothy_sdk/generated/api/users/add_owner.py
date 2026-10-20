from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.owner import Owner
from ...models.owner_add import OwnerAdd
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: OwnerAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/owners",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Owner | Problem:
    if response.status_code == 201:
        response_201 = Owner.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 422:
        response_422 = Problem.from_dict(response.json())

        return response_422

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Owner | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: OwnerAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Owner | Problem]:
    """Make a person owner

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (OwnerAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Owner | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: OwnerAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Owner | Problem | None:
    """Make a person owner

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (OwnerAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Owner | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: OwnerAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Owner | Problem]:
    """Make a person owner

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (OwnerAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Owner | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: OwnerAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Owner | Problem | None:
    """Make a person owner

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (OwnerAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Owner | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
